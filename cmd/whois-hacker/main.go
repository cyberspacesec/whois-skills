package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cyberspacesec/whois-skills/pkg/api"
	"github.com/cyberspacesec/whois-skills/pkg/metrics"
	"github.com/cyberspacesec/whois-skills/pkg/whois"
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
	// 只注册flag定义，不做其他初始化
	// 这样flag.Parse()可以在main()中调用，避免测试框架冲突
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
}

func main() {
	// 先解析命令行参数，再做其他初始化
	flag.Parse()

	// 加载YAML配置文件（如果存在），配置文件中的值作为默认值，
	// 命令行参数优先级更高
	loadConfigFromFile()

	// 配置日志（在flag.Parse()之后，确保logLevel/logFormat已被设置）
	setupLogging()

	// 打印启动信息
	logrus.Info("WhoisHacker 正在启动...")
	logrus.Infof("配置文件: %s", configFile)
	logrus.Infof("HTTP服务: %s:%d", httpHost, httpPort)

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

	// 创建API服务器（保存引用以实现优雅关闭）
	apiServer := api.NewServer(httpHost, httpPort)
	apiServer.EnableProxy = enableProxy
	apiServer.EnableCache = enableCache
	apiServer.EnableMetrics = enableMetrics
	apiServer.EnableAlerts = enableAlerts

	// 创建HTTP Server用于优雅关闭
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", httpHost, httpPort),
		Handler: apiServer.CreateHandler(),
	}

	// 在goroutine中启动HTTP服务
	go func() {
		logrus.Infof("API服务正在启动，监听地址: %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("API服务启动失败: %v", err)
		}
	}()

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logrus.Infof("收到信号: %v，开始优雅关闭...", sig)

	// 优雅关闭HTTP服务（给5秒时间完成正在处理的请求）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("HTTP服务关闭失败: %v", err)
	}

	// 导出最后的指标
	if enableMetrics {
		collector := metrics.GetCollector()
		if err := collector.ExportMetrics("data/metrics_final.json"); err != nil {
			logrus.Errorf("导出最终指标失败: %v", err)
		}
	}

	logrus.Info("服务已关闭")
}

// setupLogging 配置日志
func setupLogging() {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Warnf("解析日志级别失败: %v，使用默认级别: info", err)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

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
		Enabled:         true,
		Type:            cacheType,
		TTL:             cacheTTL,
		CleanupInterval: 300, // 5分钟清理一次
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
			cache.ClearExpired()
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

// loadConfigFromFile 从YAML配置文件加载配置
// 配置文件中的值会覆盖flag默认值，但命令行显式传入的参数优先级最高
func loadConfigFromFile() {
	if configFile == "" {
		return
	}

	cfg, err := whois.LoadYAMLConfig(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("配置文件 %s 不存在，使用默认配置", configFile)
			return
		}
		logrus.Warnf("加载配置文件失败: %v", err)
		return
	}

	// 记录哪些flag被命令行显式设置
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	// 只在命令行未显式设置时，才使用配置文件的值
	if !explicitFlags["host"] && cfg.Server.Host != "" {
		httpHost = cfg.Server.Host
	}
	if !explicitFlags["port"] && cfg.Server.Port > 0 {
		httpPort = cfg.Server.Port
	}
	if !explicitFlags["log-level"] && cfg.Log.Level != "" {
		logLevel = cfg.Log.Level
	}
	if !explicitFlags["log-format"] && cfg.Log.Format != "" {
		logFormat = cfg.Log.Format
	}
	if !explicitFlags["cache"] {
		enableCache = cfg.Cache.Enabled
	}
	if !explicitFlags["cache-type"] && cfg.Cache.Type != "" {
		cacheType = cfg.Cache.Type
	}
	if !explicitFlags["cache-ttl"] && cfg.Cache.TTL > 0 {
		cacheTTL = cfg.Cache.TTL
	}
	if !explicitFlags["cache-warmup"] {
		cacheWarmup = cfg.Cache.Warmup
	}
	if !explicitFlags["warmup-file"] && cfg.Cache.WarmupFile != "" {
		warmupFile = cfg.Cache.WarmupFile
	}
	if !explicitFlags["proxy"] {
		enableProxy = cfg.Proxy.Enabled
	}
	if !explicitFlags["proxy-file"] && cfg.Proxy.File != "" {
		proxyFile = cfg.Proxy.File
	}
	if !explicitFlags["metrics"] {
		enableMetrics = cfg.Metrics.Enabled
	}
	if !explicitFlags["metrics-interval"] && cfg.Metrics.Interval > 0 {
		metricsInterval = cfg.Metrics.Interval
	}
	if !explicitFlags["alerts"] {
		enableAlerts = cfg.Alerts.Enabled
	}
	if !explicitFlags["alerts-interval"] && cfg.Alerts.Interval > 0 {
		alertsInterval = cfg.Alerts.Interval
	}

	logrus.Infof("已从 %s 加载配置", configFile)
}
