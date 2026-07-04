package main

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/cyberspacesec/whois-skills/pkg/api"
)

// TestSetupLogging 验证日志级别与格式配置。
func TestSetupLogging(t *testing.T) {
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
		{"info/text", "info", "text", logrus.InfoLevel},
		{"debug/json", "debug", "json", logrus.DebugLevel},
		{"invalid defaults info", "invalid", "text", logrus.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagLogLevel = tt.level
			flagLogFormat = tt.format
			setupLogging()
			assert.Equal(t, tt.wantLevel, logrus.GetLevel())
			_, isJSON := logrus.StandardLogger().Formatter.(*logrus.JSONFormatter)
			if tt.format == "json" {
				assert.True(t, isJSON)
			} else {
				assert.False(t, isJSON)
			}
		})
	}
}

// TestRootCommandStructure 验证根命令注册了全部子命令。
func TestRootCommandStructure(t *testing.T) {
	root := newRootCmd()
	expected := []string{
		"serve", "version", "whois", "ip", "asn", "rdap",
		"availability", "diff", "quality", "correlation",
		"batch", "idn", "format", "export", "servers",
	}
	for _, name := range expected {
		_, _, err := root.Find([]string{name})
		assert.NoErrorf(t, err, "根命令应包含子命令 %s", name)
	}
}

// TestVersionCommand 输出应包含版本号。
func TestVersionCommand(t *testing.T) {
	original := Version
	defer func() { Version = original }()
	Version = "1.2.3"

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})
	assert.NoError(t, root.Execute())
	assert.Contains(t, buf.String(), "1.2.3")
}

// TestHTTPServerGracefulShutdown 验证 HTTP 服务的优雅关闭机制。
func TestHTTPServerGracefulShutdown(t *testing.T) {
	apiServer := api.NewServer("127.0.0.1", 0)
	apiServer.EnableCache = false
	apiServer.EnableMetrics = false
	apiServer.EnableAlerts = false
	apiServer.EnableProxy = false

	httpServer := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: apiServer.CreateHandler(),
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	time.Sleep(50 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := httpServer.Shutdown(shutdownCtx)
	assert.NoError(t, err, "HTTP服务应能优雅关闭")

	select {
	case e := <-serverErr:
		assert.Nil(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("服务关闭超时")
	}
}
