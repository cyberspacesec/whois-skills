package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyberspacesec/whois-hacker/pkg/api"
	"github.com/cyberspacesec/whois-hacker/pkg/metrics"
	"github.com/cyberspacesec/whois-hacker/pkg/whois"
	"github.com/sirupsen/logrus"
)

var (
	// 配置文件路径
	configFile string

	// HTTP服务配置
	httpHost string
	httpPort int

	// 日志配置
	logLevel  string
	logFormat string

	// 缓存配置
	enableCache    bool
	cacheType      string
	cacheTTL       int64
	cacheWarmup    bool
	warmupFile     string
	warmupInterval int64

	// 代理配置
	enableProxy bool
	proxyFile   string

	// 监控配置
	enableMetrics   bool
	metricsInterval int64

	// 告警配置
	enableAlerts   bool
	alertsInterval int64
)

func init() {
	// 解析命令行参数
	flag.StringVar(&configFile, "config", "config/config.yaml", "配置文件路径")
	flag.StringVar(&httpHost, "host", "127.0.0.1", "HTTP服务监听地址")
	flag.IntVar(&httpPort, "port", 8080, "HTTP服务监听端口")
	flag.StringVar(&logLevel, "log-level", "info", "日志级别")
	flag.StringVar(&logFormat, "log-format", "text", "日志格式 (text/json)")
	flag.BoolVar(&enableCache, "cache", true, "是否启用缓存")
	flag.StringVar(&cacheType, "cache-type", "local", "缓存类型 (local/redis)")
	flag.Int64Var(&cacheTTL, "cache-ttl", 3600, "缓存有效期（秒）")
	flag.BoolVar(&cacheWarmup, "cache-warmup", false, "是否启用缓存预热")
	flag.StringVar(&warmupFile, "warmup-file", "config/warmup.json", "预热域名列表文件")
	flag.Int64Var(&warmupInterval, "warmup-interval", 1000, "预热间隔（毫秒）")
	flag.BoolVar(&enableProxy, "proxy", false, "是否启用代理")
	flag.StringVar(&proxyFile, "proxy-file", "config/proxies.json", "代理列表文件")
	flag.BoolVar(&enableMetrics, "metrics", true, "是否启用监控")
	flag.Int64Var(&metricsInterval, "metrics-interval", 60, "监控采集间隔（秒）")
	flag.BoolVar(&enableAlerts, "alerts", true, "是否启用告警")
	flag.Int64Var(&alertsInterval, "alerts-interval", 60, "告警检查间隔（秒）")
	flag.Parse()

	// 配置日志
	setupLogging()
}

func main() {
	// 打印启动信息
	logrus.Info("WhoisHacker 正在启动...")
	logrus.Infof("配置文件: %s", configFile)
	logrus.Infof("HTTP服务: %s:%d", httpHost, httpPort)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化WHOIS服务器管理器
	serverManager := whois.GetServerManager()
	if err := serverManager.LoadFromFile("config/servers.json"); err != nil {
		logrus.Warnf("加载WHOIS服务器配置失败: %v", err)
	}

	// 初始化缓存
	if enableCache {
		setupCache()
	}

	// 初始化代理
	if enableProxy {
		setupProxy()
	}

	// 初始化监控
	if enableMetrics {
		setupMetrics()
	}

	// 初始化告警
	if enableAlerts {
		setupAlerts()
	}

	// 启动HTTP服务
	startHTTPServer()

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	sig := <-sigChan
	logrus.Infof("收到信号: %v", sig)

	// 优雅关闭
	gracefulShutdown(ctx)
}

// setupLogging 配置日志
func setupLogging() {
	// 设置日志级别
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Warnf("解析日志级别失败: %v，使用默认级别: info", err)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	// 设置日志格式
	if logFormat == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}
}

// setupCache 配置缓存
func setupCache() {
	config := whois.CacheConfig{
		Enabled: true,
		Type:    cacheType,
		TTL:     cacheTTL,
	}

	if cacheType == "redis" {
		config.RedisConfig = &whois.RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
		}
	}

	if cacheWarmup {
		config.WarmupConfig = &whois.WarmupConfig{
			Enabled:     true,
			DomainsFile: warmupFile,
			Interval:    warmupInterval,
			Concurrency: 5,
		}
	}

	cache, err := whois.NewWhoisCache(config)
	if err != nil {
		logrus.Errorf("初始化缓存失败: %v", err)
		return
	}

	logrus.Infof("缓存已启用，类型: %s", cacheType)
	if cacheWarmup {
		logrus.Info("缓存预热已启用")
	}

	// 定期清理过期缓存
	go func() {
		ticker := time.NewTicker(time.Duration(config.CleanupInterval) * time.Second)
		for range ticker.C {
			cache.Clear()
		}
	}()
}

// setupProxy 配置代理
func setupProxy() {
	if err := whois.LoadProxiesFromFile(proxyFile); err != nil {
		logrus.Errorf("加载代理配置失败: %v", err)
		return
	}
	logrus.Info("代理功能已启用")
}

// setupMetrics 配置监控
func setupMetrics() {
	collector := metrics.GetCollector()
	metrics.StartSystemMetricsCollection(time.Duration(metricsInterval) * time.Second)
	logrus.Infof("监控功能已启用，采集间隔: %ds", metricsInterval)

	// 定期导出指标
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			if err := collector.ExportMetrics("data/metrics.json"); err != nil {
				logrus.Errorf("导出指标失败: %v", err)
			}
		}
	}()
}

// setupAlerts 配置告警
func setupAlerts() {
	manager := metrics.GetAlertManager()
	manager.RegisterDefaultNotifiers()
	metrics.StartAlertManager(time.Duration(alertsInterval) * time.Second)
	logrus.Infof("告警功能已启用，检查间隔: %ds", alertsInterval)
}

// startHTTPServer 启动HTTP服务
func startHTTPServer() {
	// 创建API服务器
	server := api.NewServer(httpHost, httpPort)
	server.EnableProxy = enableProxy
	server.EnableCache = enableCache
	server.EnableMetrics = enableMetrics
	server.EnableAlerts = enableAlerts

	// 启动服务器
	if err := server.Start(); err != nil {
		logrus.Fatalf("API服务启动失败: %v", err)
	}
}

// gracefulShutdown 优雅关闭
func gracefulShutdown(ctx context.Context) {
	logrus.Info("正在关闭服务...")

	// 等待所有请求处理完成
	time.Sleep(time.Second * 5)

	// 导出最后的指标
	if enableMetrics {
		collector := metrics.GetCollector()
		if err := collector.ExportMetrics("data/metrics_final.json"); err != nil {
			logrus.Errorf("导出最终指标失败: %v", err)
		}
	}

	logrus.Info("服务已关闭")
	os.Exit(0)
}
