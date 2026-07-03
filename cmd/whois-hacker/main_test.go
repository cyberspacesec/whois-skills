package main

import (
	"bytes"
	"context"
	"flag"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cyberspacesec/whois-skills/pkg/api"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// 保存原始参数和标志集
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	// 创建新的标志集
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.String("test.testlogfile", "", "test log file")
	flag.CommandLine.Bool("test.paniconexit0", false, "panic on exit 0")

	// 设置应用标志
	configFile = "config/config.yaml"
	httpHost = "127.0.0.1"
	httpPort = 8080
	logLevel = "info"
	logFormat = "text"
	enableCache = true
	cacheType = "local"
	cacheTTL = 3600
	cacheWarmup = false
	warmupFile = "config/warmup.json"
	warmupInterval = 1000
	enableProxy = false
	proxyFile = "config/proxies.json"
	enableMetrics = true
	metricsInterval = 60
	enableAlerts = true
	alertsInterval = 60

	// 初始化测试标志
	testing.Init()

	// 解析命令行标志
	if err := flag.CommandLine.Parse([]string{}); err != nil {
		if err != flag.ErrHelp {
			logrus.Warnf("解析测试标志失败: %v", err)
		}
	}

	// 设置测试参数
	os.Args = []string{
		os.Args[0],
		"-test.testlogfile=test.log",
		"-test.paniconexit0=false",
	}

	// 运行测试
	code := m.Run()
	os.Exit(code)
}

func TestSetupLogging(t *testing.T) {
	// 保存原始日志级别和输出
	originalLevel := logrus.GetLevel()
	originalOutput := logrus.StandardLogger().Out
	defer func() {
		logrus.SetLevel(originalLevel)
		logrus.SetOutput(originalOutput)
	}()

	tests := []struct {
		name      string
		level     string
		format    string
		wantLevel logrus.Level
	}{
		{
			name:      "Valid info level with text format",
			level:     "info",
			format:    "text",
			wantLevel: logrus.InfoLevel,
		},
		{
			name:      "Valid debug level with json format",
			level:     "debug",
			format:    "json",
			wantLevel: logrus.DebugLevel,
		},
		{
			name:      "Invalid level defaults to info",
			level:     "invalid",
			format:    "text",
			wantLevel: logrus.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置测试参数
			logLevel = tt.level
			logFormat = tt.format

			// 调用setupLogging
			setupLogging()

			// 验证日志级别
			assert.Equal(t, tt.wantLevel, logrus.GetLevel())

			// 验证日志格式
			_, isJSON := logrus.StandardLogger().Formatter.(*logrus.JSONFormatter)
			if tt.format == "json" {
				assert.True(t, isJSON)
			} else {
				assert.False(t, isJSON)
			}
		})
	}
}

func TestHTTPServerGracefulShutdown(t *testing.T) {
	// 测试 http.Server.Shutdown 机制
	apiServer := api.NewServer("127.0.0.1", 0) // 端口0让系统自动分配
	apiServer.EnableCache = false
	apiServer.EnableMetrics = false
	apiServer.EnableAlerts = false
	apiServer.EnableProxy = false

	httpServer := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: apiServer.CreateHandler(),
	}

	// 在goroutine中启动HTTP服务
	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// 给服务一点启动时间
	time.Sleep(50 * time.Millisecond)

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := httpServer.Shutdown(shutdownCtx)
	assert.NoError(t, err, "HTTP服务应能优雅关闭")

	// 等待goroutine结束，验证服务确实停止了
	select {
	case e := <-serverErr:
		assert.Nil(t, e, "服务不应返回非ErrServerClosed错误")
	case <-time.After(3 * time.Second):
		t.Fatal("服务关闭超时")
	}
}

func TestSetupProxy(t *testing.T) {
	// 保存原始配置
	originalEnableProxy := enableProxy
	originalProxyFile := proxyFile
	defer func() {
		enableProxy = originalEnableProxy
		proxyFile = originalProxyFile
	}()

	// 测试不存在的代理文件
	enableProxy = true
	proxyFile = "nonexistent_proxies.json"

	// setupProxy不应panic，只记录错误
	assert.NotPanics(t, setupProxy)
}

func TestSetupCache(t *testing.T) {
	// 保存原始配置
	originalEnableCache := enableCache
	originalCacheType := cacheType
	originalCacheTTL := cacheTTL
	originalCacheWarmup := cacheWarmup
	defer func() {
		enableCache = originalEnableCache
		cacheType = originalCacheType
		cacheTTL = originalCacheTTL
		cacheWarmup = originalCacheWarmup
	}()

	tests := []struct {
		name       string
		cacheType  string
		cacheTTL   int64
		warmup     bool
		wantOutput string
	}{
		{
			name:       "Local cache without warmup",
			cacheType:  "local",
			cacheTTL:   3600,
			warmup:     false,
			wantOutput: "缓存已启用，类型: local",
		},
		{
			name:       "Local cache with warmup",
			cacheType:  "local",
			cacheTTL:   3600,
			warmup:     true,
			wantOutput: "缓存预热已启用",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置测试参数
			enableCache = true
			cacheType = tt.cacheType
			cacheTTL = tt.cacheTTL
			cacheWarmup = tt.warmup

			// 捕获日志输出
			var buf bytes.Buffer
			logrus.SetOutput(&buf)

			// 调用setupCache
			setupCache()

			// 检查日志输出
			output := buf.String()
			assert.Contains(t, output, tt.wantOutput)
		})
	}
}
