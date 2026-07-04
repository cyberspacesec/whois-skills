package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/cyberspacesec/whois-skills/pkg/api"
	"github.com/cyberspacesec/whois-skills/pkg/metrics"
	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// serve 子命令专用 flag
var (
	serveHost       string
	servePort       int
	serveCache      bool
	serveCacheType  string
	serveCacheTTL   int64
	serveCacheWarmup bool
	serveWarmupFile string
	serveMetrics    bool
	serveMetricsInt int64
	serveAlerts     bool
	serveAlertsInt  int64
)

// newServeCmd 启动 HTTP API 服务（原 main.go 的核心逻辑）。
func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动 HTTP API 服务",
		Long: `启动 whois-hacker 的 HTTP API 服务，暴露 WHOIS/IP/ASN/RDAP/批量/
关联等全部 HTTP 端点，以及 MCP 任务流端点。

服务常驻运行，按 Ctrl+C 或收到 SIGTERM/SIGINT 后优雅关闭（5s 超时）。

示例：
  whois-hacker serve --host 0.0.0.0 --port 8080
  whois-hacker serve --cache-type redis --log-format json`,
		RunE: runServe,
	}

	f := cmd.Flags()
	f.StringVar(&serveHost, "host", "127.0.0.1", "HTTP 服务监听地址")
	f.IntVar(&servePort, "port", 8080, "HTTP 服务监听端口")
	f.BoolVar(&serveCache, "cache", true, "是否启用缓存")
	f.StringVar(&serveCacheType, "cache-type", "local", "缓存类型 (local/redis)")
	f.Int64Var(&serveCacheTTL, "cache-ttl", 3600, "缓存有效期（秒）")
	f.BoolVar(&serveCacheWarmup, "cache-warmup", false, "是否启用缓存预热")
	f.StringVar(&serveWarmupFile, "warmup-file", "config/warmup.json", "预热域名列表文件")
	f.BoolVar(&serveMetrics, "metrics", true, "是否启用监控")
	f.Int64Var(&serveMetricsInt, "metrics-interval", 60, "监控采集间隔（秒）")
	f.BoolVar(&serveAlerts, "alerts", true, "是否启用告警")
	f.Int64Var(&serveAlertsInt, "alerts-interval", 60, "告警检查间隔（秒）")

	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	// 用 YAML 配置文件覆盖 serve flag（仅当命令行未显式设置时）
	applyServeConfigFromYAML(cmd)

	logrus.Info("WhoisHacker 正在启动...")
	logrus.Infof("HTTP服务: %s:%d", serveHost, servePort)

	// 初始化缓存
	if serveCache {
		setupCache()
	}

	// 初始化监控
	if serveMetrics {
		setupMetrics()
	}

	// 初始化告警
	if serveAlerts {
		setupAlerts()
	}

	// 创建 API 服务器
	apiServer := api.NewServer(serveHost, servePort)
	apiServer.EnableProxy = flagUseProxy
	apiServer.EnableCache = serveCache
	apiServer.EnableMetrics = serveMetrics
	apiServer.EnableAlerts = serveAlerts

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", serveHost, servePort),
		Handler: apiServer.CreateHandler(),
	}

	// 在 goroutine 中启动 HTTP 服务
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

	// 优雅关闭 HTTP 服务（5s 超时）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("HTTP服务关闭失败: %v", err)
	}

	// 导出最后的指标
	if serveMetrics {
		collector := metrics.GetCollector()
		if err := collector.ExportMetrics("data/metrics_final.json"); err != nil {
			logrus.Errorf("导出最终指标失败: %v", err)
		}
	}

	logrus.Info("服务已关闭")
	return nil
}

// setupCache 配置缓存（serve 专用）。
func setupCache() {
	config := whois.CacheConfig{
		Enabled:         true,
		Type:            serveCacheType,
		TTL:             serveCacheTTL,
		CleanupInterval: 300,
	}

	if serveCacheType == "redis" {
		config.RedisConfig = &whois.RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
		}
	}

	if serveCacheWarmup {
		config.WarmupConfig = &whois.WarmupConfig{
			Enabled:     true,
			DomainsFile: serveWarmupFile,
			Interval:    1000,
			Concurrency: 5,
		}
	}

	cache, err := whois.NewWhoisCache(config)
	if err != nil {
		logrus.Errorf("初始化缓存失败: %v", err)
		return
	}
	_ = cache // 缓存已注册到全局

	logrus.Infof("缓存已启用，类型: %s", serveCacheType)
	if serveCacheWarmup {
		logrus.Info("缓存预热已启用")
	}
}

// setupMetrics 配置监控（serve 专用）。
func setupMetrics() {
	collector := metrics.GetCollector()
	metrics.StartSystemMetricsCollection(time.Duration(serveMetricsInt) * time.Second)
	logrus.Infof("监控功能已启用，采集间隔: %ds", serveMetricsInt)
	_ = collector
}

// setupAlerts 配置告警（serve 专用）。
func setupAlerts() {
	manager := metrics.GetAlertManager()
	manager.RegisterDefaultNotifiers()
	metrics.StartAlertManager(time.Duration(serveAlertsInt) * time.Second)
	logrus.Infof("告警功能已启用，检查间隔: %ds", serveAlertsInt)
}

// applyServeConfigFromYAML 用 YAML 配置文件覆盖 serve 子命令的 flag 值。
// 仅当对应 flag 未在命令行显式设置时才覆盖（命令行优先级最高）。
func applyServeConfigFromYAML(cmd *cobra.Command) {
	if flagConfig == "" {
		return
	}
	cfg, err := whois.LoadYAMLConfig(flagConfig)
	if err != nil {
		return // 文件不存在或解析失败已在 loadConfigFromFile 中处理
	}

	flags := cmd.Flags()
	setIfNotChanged := func(name string, setter func()) {
		if !flags.Changed(name) {
			setter()
		}
	}

	setIfNotChanged("host", func() {
		if cfg.Server.Host != "" {
			serveHost = cfg.Server.Host
		}
	})
	setIfNotChanged("port", func() {
		if cfg.Server.Port > 0 {
			servePort = cfg.Server.Port
		}
	})
	setIfNotChanged("cache", func() { serveCache = cfg.Cache.Enabled })
	setIfNotChanged("cache-type", func() {
		if cfg.Cache.Type != "" {
			serveCacheType = cfg.Cache.Type
		}
	})
	setIfNotChanged("cache-ttl", func() {
		if cfg.Cache.TTL > 0 {
			serveCacheTTL = cfg.Cache.TTL
		}
	})
	setIfNotChanged("cache-warmup", func() { serveCacheWarmup = cfg.Cache.Warmup })
	setIfNotChanged("warmup-file", func() {
		if cfg.Cache.WarmupFile != "" {
			serveWarmupFile = cfg.Cache.WarmupFile
		}
	})
	setIfNotChanged("metrics", func() { serveMetrics = cfg.Metrics.Enabled })
	setIfNotChanged("metrics-interval", func() {
		if cfg.Metrics.Interval > 0 {
			serveMetricsInt = cfg.Metrics.Interval
		}
	})
	setIfNotChanged("alerts", func() { serveAlerts = cfg.Alerts.Enabled })
	setIfNotChanged("alerts-interval", func() {
		if cfg.Alerts.Interval > 0 {
			serveAlertsInt = cfg.Alerts.Interval
		}
	})
}
