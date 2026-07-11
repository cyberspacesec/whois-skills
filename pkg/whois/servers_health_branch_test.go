package whois

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== servers.go startHealthCheck 循环体 ====================
// startHealthCheck 是无 ctx 取消的 for-range ticker.C 无限循环（line 198-223），
// 无 Stop 入口，goroutine 会泄漏。这里用极短 interval 启动一次，等若干 tick
// 覆盖循环体（servers 收集 / wg / logHealthStatus），然后让测试退出（进程退出
// 时清理泄漏 goroutine）。

// TestStartHealthCheck_EmptyServersTick 空 servers → 循环体执行但 checkServerHealth
// 不被调用（覆盖 line 202-208、219-222；212-214 不执行）。
func TestStartHealthCheck_EmptyServersTick(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		healthCheckInterval: 5 * time.Millisecond,
		healthCheckTimeout:  50 * time.Millisecond,
		maxFailures:         3,
	}
	go mgr.startHealthCheck()
	// 等若干 tick，确保循环体至少执行一次
	time.Sleep(60 * time.Millisecond)
	// 空 servers 不应产生健康记录
	mgr.mu.RLock()
	empty := len(mgr.serverHealth) == 0
	mgr.mu.RUnlock()
	assert.True(t, empty, "空 servers 不应产生健康记录")
}

// TestStartHealthCheck_NonEmptyServersTick servers 非空 → 启动 checkServerHealth
// goroutine（覆盖 line 212-214）。用一个不可达地址触发 Dial 失败（失败分支已覆盖）。
func TestStartHealthCheck_NonEmptyServersTick(t *testing.T) {
	mgr := &WhoisServerManager{
		servers: map[string]string{
			"com": "whois.invalid.tld.unreachable.example",
		},
		serverHealth:        make(map[string]*ServerHealth),
		healthCheckInterval: 5 * time.Millisecond,
		healthCheckTimeout:  30 * time.Millisecond,
		maxFailures:         3,
	}
	go mgr.startHealthCheck()
	// 等若干 tick，确保 checkServerHealth 至少执行一次并写入 health 记录
	time.Sleep(90 * time.Millisecond)
	mgr.mu.RLock()
	_, hasCom := mgr.serverHealth["whois.invalid.tld.unreachable.example"]
	mgr.mu.RUnlock()
	assert.True(t, hasCom, "非空 servers 应触发 checkServerHealth 并创建健康记录")
}
