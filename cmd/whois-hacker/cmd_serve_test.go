package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- 辅助 ---

// freePort 返回一个当前可用的 TCP 端口。
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// withServeDefaults 保存并最终恢复全部 serve 相关全局 flag。
func withServeDefaults(t *testing.T) {
	t.Helper()
	orig := struct {
		host, cacheType, warmupFile string
		port                         int
		cache                        bool
		cacheTTL                     int64
		cacheWarmup                  bool
		metrics, alerts              bool
		metricsInt, alertsInt        int64
		useProxy                     bool
	}{
		host: serveHost, cacheType: serveCacheType, warmupFile: serveWarmupFile,
		port: servePort, cache: serveCache, cacheTTL: serveCacheTTL,
		cacheWarmup: serveCacheWarmup, metrics: serveMetrics, alerts: serveAlerts,
		metricsInt: serveMetricsInt, alertsInt: serveAlertsInt,
		useProxy: flagUseProxy,
	}
	t.Cleanup(func() {
		serveHost, serveCacheType, serveWarmupFile = orig.host, orig.cacheType, orig.warmupFile
		servePort, serveCache, serveCacheTTL = orig.port, orig.cache, orig.cacheTTL
		serveCacheWarmup, serveMetrics, serveAlerts = orig.cacheWarmup, orig.metrics, orig.alerts
		serveMetricsInt, serveAlertsInt, flagUseProxy = orig.metricsInt, orig.alertsInt, orig.useProxy
	})
}

// resetServeFlagsUnchanged 把 serve 相关 flag 的 Changed 标记全部清零，
// 模拟命令行未显式设置，允许 YAML 覆盖。
func resetServeFlagsUnchanged(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	for _, name := range []string{
		"host", "port", "cache", "cache-type", "cache-ttl",
		"cache-warmup", "warmup-file", "metrics", "metrics-interval",
		"alerts", "alerts-interval",
	} {
		if fl := cmd.Flags().Lookup(name); fl != nil {
			fl.Changed = false
		}
	}
}

// waitForServeReady 轮询探测端口直到 HTTP 监听起来（最多 ~5s）。
// 若 runServe 在此期间提前结束（done 有值），返回该错误。
func waitForServeReady(t *testing.T, port int, done <-chan error) bool {
	t.Helper()
	for i := 0; i < 50; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		select {
		case err := <-done:
			t.Fatalf("runServe 提前结束且未监听: %v", err)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// signalAndServe 在确认监听后发 SIGINT 触发优雅关闭，并在 10s 内等待 runServe 返回。
func signalAndServe(t *testing.T, port int, done <-chan error) {
	t.Helper()
	if !waitForServeReady(t, port, done) {
		t.Fatalf("runServe 在 5s 内未监听端口 %d", port)
	}
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runServe 10s 内未优雅退出")
	}
}

// --- newServeCmd ---

// TestNewServeCmd 验证 serve 子命令的注册、默认 flag 值与命令行解析。
func TestNewServeCmd(t *testing.T) {
	withServeDefaults(t)
	cmd := newServeCmd()

	assert.Equal(t, "serve", cmd.Use)
	assert.Equal(t, "启动 HTTP API 服务", cmd.Short)
	assert.NotNil(t, cmd.RunE)

	// 默认值
	assert.Equal(t, "127.0.0.1", serveHost)
	assert.Equal(t, 8080, servePort)
	assert.True(t, serveCache)
	assert.Equal(t, "local", serveCacheType)
	assert.Equal(t, int64(3600), serveCacheTTL)
	assert.False(t, serveCacheWarmup)
	assert.Equal(t, "config/warmup.json", serveWarmupFile)
	assert.True(t, serveMetrics)
	assert.Equal(t, int64(60), serveMetricsInt)
	assert.True(t, serveAlerts)
	assert.Equal(t, int64(60), serveAlertsInt)

	// 命令行参数解析覆盖默认值
	require.NoError(t, cmd.Flags().Parse([]string{
		"--host", "0.0.0.0", "--port", "9090",
		"--cache=false", "--cache-type", "redis", "--cache-ttl", "120",
		"--cache-warmup=true", "--warmup-file", "/tmp/w.json",
		"--metrics=false", "--metrics-interval", "7",
		"--alerts=false", "--alerts-interval", "9",
	}))
	assert.Equal(t, "0.0.0.0", serveHost)
	assert.Equal(t, 9090, servePort)
	assert.False(t, serveCache)
	assert.Equal(t, "redis", serveCacheType)
	assert.Equal(t, int64(120), serveCacheTTL)
	assert.True(t, serveCacheWarmup)
	assert.Equal(t, "/tmp/w.json", serveWarmupFile)
	assert.False(t, serveMetrics)
	assert.Equal(t, int64(7), serveMetricsInt)
	assert.False(t, serveAlerts)
	assert.Equal(t, int64(9), serveAlertsInt)
}

// --- setupCache ---

// TestSetupCache_Local 成功初始化本地缓存。
func TestSetupCache_Local(t *testing.T) {
	withServeDefaults(t)
	serveCacheType = "local"
	serveCacheWarmup = false
	assert.NotPanics(t, func() { setupCache() })
}

// TestSetupCache_LocalWithWarmup 本地缓存 + 预热配置分支。
func TestSetupCache_LocalWithWarmup(t *testing.T) {
	withServeDefaults(t)
	serveCacheType = "local"
	serveCacheWarmup = true
	serveWarmupFile = "config/warmup.json"
	assert.NotPanics(t, func() { setupCache() })
}

// TestSetupCache_RedisFail serveCacheType=redis 但 localhost:6379 不可达，
// 走 NewWhoisCache 失败分支（logrus.Errorf + return）。
func TestSetupCache_RedisFail(t *testing.T) {
	withServeDefaults(t)
	serveCacheType = "redis"
	serveCacheWarmup = false
	assert.NotPanics(t, func() { setupCache() })
}

// TestSetupCache_RedisFailWithWarmup redis 失败 + warmup=true，覆盖两个分支同时命中的路径。
func TestSetupCache_RedisFailWithWarmup(t *testing.T) {
	withServeDefaults(t)
	serveCacheType = "redis"
	serveCacheWarmup = true
	serveWarmupFile = "config/warmup.json"
	assert.NotPanics(t, func() { setupCache() })
}

// TestSetupCache_RedisOK serveCacheType=redis 且能连上（miniredis 抢占 6379 端口）。
// 覆盖 NewWhoisCache 成功 + redis warmup 日志分支。
func TestSetupCache_RedisOK(t *testing.T) {
	withServeDefaults(t)

	mr := miniredis.NewMiniRedis()
	if err := mr.StartAddr("127.0.0.1:6379"); err != nil {
		t.Skipf("无法占用 6379 端口启动 miniredis（%v），跳过 redis 成功分支", err)
	}
	t.Cleanup(mr.Close)

	serveCacheType = "redis"
	serveCacheWarmup = true
	serveWarmupFile = "config/warmup.json"
	assert.NotPanics(t, func() { setupCache() })
}

// TestSetupCache_RedisOKNoWarmup redis 成功 + 不预热，覆盖不带 warmup 日志分支。
func TestSetupCache_RedisOKNoWarmup(t *testing.T) {
	withServeDefaults(t)

	mr := miniredis.NewMiniRedis()
	if err := mr.StartAddr("127.0.0.1:6379"); err != nil {
		t.Skipf("无法占用 6379 端口启动 miniredis（%v），跳过", err)
	}
	t.Cleanup(mr.Close)

	serveCacheType = "redis"
	serveCacheWarmup = false
	assert.NotPanics(t, func() { setupCache() })
}

// --- setupMetrics ---

// TestSetupMetrics 启用监控，验证不 panic 并采集间隔生效。
func TestSetupMetrics(t *testing.T) {
	withServeDefaults(t)
	serveMetricsInt = 1
	assert.NotPanics(t, func() { setupMetrics() })
}

// --- setupAlerts ---

// TestSetupAlerts 启用告警，验证不 panic。
func TestSetupAlerts(t *testing.T) {
	withServeDefaults(t)
	serveAlertsInt = 1
	assert.NotPanics(t, func() { setupAlerts() })
}

// --- applyServeConfigFromYAML ---

// TestApplyServeConfigFromYAML_NoConfig flagConfig 为空时立即返回，不改任何 flag。
func TestApplyServeConfigFromYAML_NoConfig(t *testing.T) {
	withServeDefaults(t)
	origConfig := flagConfig
	t.Cleanup(func() { flagConfig = origConfig })
	flagConfig = ""

	cmd := newServeCmd()
	serveHost = "9.9.9.9"
	servePort = 9999
	applyServeConfigFromYAML(cmd)
	assert.Equal(t, "9.9.9.9", serveHost)
	assert.Equal(t, 9999, servePort)
}

// TestApplyServeConfigFromYAML_LoadFail 加载失败（文件不存在）时直接返回，不改 flag。
func TestApplyServeConfigFromYAML_LoadFail(t *testing.T) {
	withServeDefaults(t)
	origConfig := flagConfig
	t.Cleanup(func() { flagConfig = origConfig })
	flagConfig = "/definitely/not/exists/serve.yaml"

	cmd := newServeCmd()
	serveHost = "9.9.9.9"
	servePort = 9999
	applyServeConfigFromYAML(cmd)
	assert.Equal(t, "9.9.9.9", serveHost)
	assert.Equal(t, 9999, servePort)
}

// TestApplyServeConfigFromYAML_BadYAML 坏 YAML 走解析失败分支。
func TestApplyServeConfigFromYAML_BadYAML(t *testing.T) {
	withServeDefaults(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("  : : not: valid: yaml: ["), 0644))

	origConfig := flagConfig
	t.Cleanup(func() { flagConfig = origConfig })
	flagConfig = path

	cmd := newServeCmd()
	serveHost = "9.9.9.9"
	applyServeConfigFromYAML(cmd)
	assert.Equal(t, "9.9.9.9", serveHost) // 加载失败，未覆盖
}

// TestApplyServeConfigFromYAML_Override 合法 YAML 且命令行未显式设置时覆盖全部 serve flag。
func TestApplyServeConfigFromYAML_Override(t *testing.T) {
	withServeDefaults(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "serve.yaml")
	content := `
server:
  host: "1.2.3.4"
  port: 7777
cache:
  enabled: false
  type: redis
  ttl: 120
  warmup: true
  warmup_file: /tmp/w.json
metrics:
  enabled: false
  interval: 5
alerts:
  enabled: false
  interval: 7
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	origConfig := flagConfig
	t.Cleanup(func() { flagConfig = origConfig })
	flagConfig = path

	cmd := newServeCmd()
	resetServeFlagsUnchanged(t, cmd)

	applyServeConfigFromYAML(cmd)

	assert.Equal(t, "1.2.3.4", serveHost)
	assert.Equal(t, 7777, servePort)
	assert.False(t, serveCache)
	assert.Equal(t, "redis", serveCacheType)
	assert.Equal(t, int64(120), serveCacheTTL)
	assert.True(t, serveCacheWarmup)
	assert.Equal(t, "/tmp/w.json", serveWarmupFile)
	assert.False(t, serveMetrics)
	assert.Equal(t, int64(5), serveMetricsInt)
	assert.False(t, serveAlerts)
	assert.Equal(t, int64(7), serveAlertsInt)
}

// TestApplyServeConfigFromYAML_PartialYAML 部分字段为空/零值时不覆盖（走各 if 守卫）。
func TestApplyServeConfigFromYAML_PartialYAML(t *testing.T) {
	withServeDefaults(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	// 只设 host/port/enabled，其余字段留空，走各 if 守卫不覆盖分支。
	content := `
server:
  host: "5.6.7.8"
  port: 8888
cache:
  enabled: true
  type: ""
  ttl: 0
  warmup: false
  warmup_file: ""
metrics:
  enabled: true
  interval: 0
alerts:
  enabled: true
  interval: 0
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	origConfig := flagConfig
	t.Cleanup(func() { flagConfig = origConfig })
	flagConfig = path

	cmd := newServeCmd()
	// 预置非默认值，验证部分字段不被空/零值覆盖。
	serveCacheType = "local"
	serveCacheTTL = 3600
	serveWarmupFile = "config/warmup.json"
	serveMetricsInt = 60
	serveAlertsInt = 60
	resetServeFlagsUnchanged(t, cmd)

	applyServeConfigFromYAML(cmd)

	assert.Equal(t, "5.6.7.8", serveHost)
	assert.Equal(t, 8888, servePort)
	assert.True(t, serveCache)
	// 空/零值字段保持原值
	assert.Equal(t, "local", serveCacheType)
	assert.Equal(t, int64(3600), serveCacheTTL)
	assert.False(t, serveCacheWarmup)
	assert.Equal(t, "config/warmup.json", serveWarmupFile)
	assert.True(t, serveMetrics)
	assert.Equal(t, int64(60), serveMetricsInt)
	assert.True(t, serveAlerts)
	assert.Equal(t, int64(60), serveAlertsInt)
}

// TestApplyServeConfigFromYAML_FlagChanged 命令行显式设置时不被 YAML 覆盖。
func TestApplyServeConfigFromYAML_FlagChanged(t *testing.T) {
	withServeDefaults(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "override.yaml")
	content := `
server:
  host: "1.2.3.4"
  port: 7777
cache:
  enabled: false
  type: redis
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	origConfig := flagConfig
	t.Cleanup(func() { flagConfig = origConfig })
	flagConfig = path

	cmd := newServeCmd()
	// 预置命令行值
	serveHost = "cli.host"
	servePort = 11111
	serveCache = true
	serveCacheType = "local"
	// 标记为已改变（模拟命令行显式设置）
	for _, name := range []string{"host", "port", "cache", "cache-type"} {
		if fl := cmd.Flags().Lookup(name); fl != nil {
			fl.Changed = true
		}
	}

	applyServeConfigFromYAML(cmd)

	assert.Equal(t, "cli.host", serveHost) // 未被覆盖
	assert.Equal(t, 11111, servePort)
	assert.True(t, serveCache)
	assert.Equal(t, "local", serveCacheType)
}

// --- runServe ---

// TestRunServe_Minimal 最小化组件路径（关缓存/监控/告警），端口 0 随机端口，
// goroutine 启动后向自己发 SIGINT，验证优雅关闭主路径（信号处理 + Shutdown）。
func TestRunServe_Minimal(t *testing.T) {
	withServeDefaults(t)

	port := freePort(t)
	flagUseProxy = false

	// newServeCmd 会用 flag 默认值重置 serveHost/servePort 等全局变量，
	// 因此必须在 newServeCmd 之后再覆盖这些变量，否则端口仍为默认 8080。
	cmd := newServeCmd()
	serveHost = "127.0.0.1"
	servePort = port
	serveCache = false
	serveMetrics = false
	serveAlerts = false

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("runServe panic: %v\n%s", r, debug.Stack())
			}
		}()
		done <- runServe(cmd, nil)
	}()

	signalAndServe(t, port, done)
}

// TestRunServe_FullStack 全组件路径（cache=local + metrics + alerts），
// 覆盖 setupCache/setupMetrics/setupAlerts 调用分支以及关闭时 ExportMetrics 分支。
func TestRunServe_FullStack(t *testing.T) {
	withServeDefaults(t)

	port := freePort(t)
	flagUseProxy = false

	cmd := newServeCmd()
	serveHost = "127.0.0.1"
	servePort = port

	serveCache = true
	serveCacheType = "local"
	serveCacheWarmup = false
	serveMetrics = true
	serveMetricsInt = 1
	serveAlerts = true
	serveAlertsInt = 1

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("runServe panic: %v", r)
			}
		}()
		done <- runServe(cmd, nil)
	}()

	signalAndServe(t, port, done)
}

// TestRunServe_RedisCache serveCacheType=redis 但无可用 redis 的全栈启动路径：
// setupCache 走 redis 失败分支后仍继续启动 HTTP 并优雅关闭。
func TestRunServe_RedisCache(t *testing.T) {
	withServeDefaults(t)

	port := freePort(t)
	flagUseProxy = false

	cmd := newServeCmd()
	serveHost = "127.0.0.1"
	servePort = port

	serveCache = true
	serveCacheType = "redis" // localhost:6379 不可达，setupCache 内部仅记错误日志
	serveCacheWarmup = false
	serveMetrics = false
	serveAlerts = false

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("runServe panic: %v", r)
			}
		}()
		done <- runServe(cmd, nil)
	}()

	signalAndServe(t, port, done)
}

// TestRunServe_ListenFail 预先占用目标端口，runServe 的 goroutine 中 ListenAndServe
// 失败会调 logrus.Fatalf。通过替换 logrus.StandardLogger().ExitFunc 阻止 os.Exit，
// 并在 ExitFunc 中发 SIGINT 让阻塞在 sigChan 的主流程继续，从而覆盖启动失败分支。
func TestRunServe_ListenFail(t *testing.T) {
	withServeDefaults(t)

	port := freePort(t)
	// 先占用该端口，使 runServe 内的 ListenAndServe 失败。
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer l.Close()

	cmd := newServeCmd()
	serveHost = "127.0.0.1"
	servePort = port
	flagUseProxy = false
	serveCache = false
	serveMetrics = false
	serveAlerts = false

	std := logrus.StandardLogger()
	var mu sync.Mutex
	fatalfCalled := false
	var buf bytes.Buffer
	origExit := std.ExitFunc
	origOut := std.Out
	t.Cleanup(func() {
		std.ExitFunc = origExit
		std.SetOutput(origOut)
	})
	std.SetOutput(&buf)
	std.ExitFunc = func(code int) {
		mu.Lock()
		fatalfCalled = true
		mu.Unlock()
		// 发 SIGINT 让 runServe 主流程跳出 sigChan 等待继续优雅关闭。
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("runServe panic: %v", r)
				return
			}
			done <- nil
		}()
		_ = runServe(cmd, nil)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runServe 10s 内未退出")
	}

	mu.Lock()
	called := fatalfCalled
	mu.Unlock()
	assert.True(t, called, "应触发 logrus.Fatalf 启动失败分支")
	assert.Contains(t, buf.String(), "API服务启动失败")
}

// TestRunServe_ShutdownFail 覆盖 runServe 中 httpServer.Shutdown 返回错误的分支（line 120）。
// 策略：runServe 监听后，建立一条占住的 TCP 连接（不发送完整 HTTP 请求、不主动关闭），
// http.Server 默认无读超时，会一直阻塞该连接；Shutdown 在 5s ctx 超时后返回 DeadlineExceeded，
// 从而命中 logrus.Errorf("HTTP服务关闭失败: ...") 分支。本测试约耗时 5s+。
func TestRunServe_ShutdownFail(t *testing.T) {
	withServeDefaults(t)

	port := freePort(t)
	flagUseProxy = false

	cmd := newServeCmd()
	serveHost = "127.0.0.1"
	servePort = port
	serveCache = false
	serveMetrics = false
	serveAlerts = false

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("runServe panic: %v", r)
				return
			}
			done <- nil
		}()
		_ = runServe(cmd, nil)
	}()

	if !waitForServeReady(t, port, done) {
		t.Fatalf("runServe 在 5s 内未监听端口 %d", port)
	}

	// 建立并保持一条不完整的 TCP 连接，使 Shutdown 无法在 5s 内完成。
	stickyConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	defer stickyConn.Close()
	// 只写一小段但不写完整 HTTP 请求也不关闭，http.Server 会一直等待读。
	_, _ = stickyConn.Write([]byte("GET / HTTP/1.1\r\n"))

	// 发 SIGINT 触发优雅关闭：Shutdown 会在 5s 超时后返回 DeadlineExceeded。
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("runServe 15s 内未退出（Shutdown 超时分支未触发）")
	}
}
