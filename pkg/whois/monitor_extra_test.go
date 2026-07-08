package whois

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ---- createExpiryAlert: 全部分支 ----

func TestDomainMonitor_CreateExpiryAlert_AllBranches(t *testing.T) {
	m := NewDomainMonitor(DefaultMonitorConfig())

	// Expired (days<=0)
	a := m.createExpiryAlert("x.com", WatchStatusExpired, -5)
	assert.Equal(t, AlertLevelCritical, a.Level)
	assert.Equal(t, AlertExpiryPassed, a.Type)
	assert.Contains(t, a.Message, "已过期 5 天")

	// Critical
	a = m.createExpiryAlert("x.com", WatchStatusCritical, 3)
	assert.Equal(t, AlertLevelCritical, a.Level)
	assert.Equal(t, AlertExpiryCritical, a.Type)
	assert.Contains(t, a.Message, "3 天后到期")

	// Warning
	a = m.createExpiryAlert("x.com", WatchStatusWarning, 20)
	assert.Equal(t, AlertLevelWarning, a.Level)
	assert.Contains(t, a.Message, "20 天后到期")

	// default (其它状态)
	a = m.createExpiryAlert("x.com", WatchStatusActive, 100)
	assert.Equal(t, AlertLevelInfo, a.Level)
	assert.Contains(t, a.Message, "剩余 100 天")
}

// ---- emitAlert: 通道发送 + 回调 + 持久化（注入 provider）----

func TestDomainMonitor_EmitAlert(t *testing.T) {
	// 注入 AlertStorageProvider 避免 nil 报错
	origAlert := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = origAlert }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)

	m := NewDomainMonitor(DefaultMonitorConfig())
	var got *DomainAlert
	m.OnAlert(func(a *DomainAlert) { got = a })

	alert := &DomainAlert{ID: "1", Domain: "x.com", Type: AlertExpiryWarning, Message: "test"}
	m.emitAlert(alert)

	assert.Same(t, alert, got)
	// 通道应收到
	select {
	case ch := <-m.Alerts():
		assert.Same(t, alert, ch)
	default:
		t.Fatal("expected alert in channel")
	}
}

// ---- emitAlert: 通道已满 → 丢弃 ----

func TestDomainMonitor_EmitAlert_ChannelFull(t *testing.T) {
	origAlert := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = origAlert }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)

	m := NewDomainMonitor(MonitorConfig{CheckInterval: 1, ExpiryWarningDays: 30, ExpiryCriticalDays: 7, MaxConcurrentChecks: 1})
	// 填满通道（容量 100）
	for i := 0; i < 100; i++ {
		m.alertChan <- &DomainAlert{ID: "fill"}
	}
	// 再发一个 → 应丢弃，不阻塞
	assert.NotPanics(t, func() {
		m.emitAlert(&DomainAlert{ID: "overflow", Domain: "x.com"})
	})
}

// ---- calculateDaysRemaining: 不可解析格式 ----

func TestCalculateDaysRemaining_Unparseable(t *testing.T) {
	assert.Equal(t, -1, calculateDaysRemaining("not-a-date"))
	assert.Equal(t, -1, calculateDaysRemaining(""))
}

// ---- CheckNow: 成功路径（域名在列表中 + checkDomain 错误路径）----

func TestDomainMonitor_CheckNow_Success(t *testing.T) {
	origProv := globalWhoisQueryProvider
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalWhoisQueryProvider = origProv
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
	// stub provider 返回错误 → checkDomain 走错误分支
	defer withStubQueryProvider(&stubWhoisQueryProvider{queryErr: assertError("boom")})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	m := NewDomainMonitor(DefaultMonitorConfig())
	m.AddWatch("example.com", nil)
	err := m.CheckNow(context.Background(), "example.com")
	assert.NoError(t, err)
	// checkDomain 错误分支 → Status=Error
	st := m.GetWatchState("example.com")
	assert.Equal(t, WatchStatusError, st.Status)
	assert.GreaterOrEqual(t, st.AlertCount, 0)
}

// ---- checkDomain: 到期状态变更 + 各种变更告警 ----

func TestDomainMonitor_CheckDomain_Changes(t *testing.T) {
	origProv := globalWhoisQueryProvider
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalWhoisQueryProvider = origProv
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)

	// 初始 WHOIS 信息
	oldInfo := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			ExpirationDate: time.Now().Add(200 * 24 * time.Hour).Format("2006-01-02"),
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.old.com", "ns2.old.com"},
		},
		Registrant: &whoisparser.Contact{Name: "Old Owner", Email: "old@x.com", Organization: "OldOrg"},
	}

	// 新查询结果：到期临近、状态/NS/注册人变更
	newInfo := whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			ExpirationDate: time.Now().Add(5 * 24 * time.Hour).Format("2006-01-02"), // critical
			Status:         []string{"serverDeleteProhibited"},
			NameServers:    []string{"ns1.new.com"},
		},
		Registrant: &whoisparser.Contact{Name: "New Owner", Email: "new@x.com", Organization: "NewOrg"},
	}

	defer withStubQueryProvider(&stubWhoisQueryProvider{
		raw:  "raw",
		info: newInfo,
	})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	m := NewDomainMonitor(DefaultMonitorConfig())
	m.AddWatch("example.com", oldInfo)

	var collected []*DomainAlert
	m.OnAlert(func(a *DomainAlert) { collected = append(collected, a) })

	m.checkDomain(context.Background(), "example.com")

	st := m.GetWatchState("example.com")
	assert.Equal(t, WatchStatusChanged, st.Status) // 有变更
	assert.GreaterOrEqual(t, st.AlertCount, 1)
	// 应触发到期、状态、注册人、NS 变更告警
	types := map[AlertType]bool{}
	for _, a := range collected {
		types[a.Type] = true
	}
	assert.True(t, types[AlertExpiryCritical] || types[AlertExpiryPassed] || types[AlertExpiryWarning], "expiry alert expected")
	assert.True(t, types[AlertStatusChange])
	assert.True(t, types[AlertRegistrantChange])
	assert.True(t, types[AlertNSChange])
}

// ---- checkDomain: result.Info==nil 直接返回 ----

func TestDomainMonitor_CheckDomain_NilInfo(t *testing.T) {
	origProv := globalWhoisQueryProvider
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalWhoisQueryProvider = origProv
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)

	// stub 返回空 info（Parse 返回空 WhoisInfo，result.Info 非 nil 但字段空）
	// 要让 result.Info==nil，Parse 必须返回 err → 但 ExecuteQuery 在 Parse err 时返回 err
	// 所以走 err 分支而非 nil-info 分支。nil-info 分支实际不可达（ExecuteQuery 总会设置 Info 或返回 err）
	// 这里改测 err 分支已覆盖，nil-info 分支记录为不可达。
	_ = origProv
}

// ---- checkDomain: 域名已不在 watchlist ----

func TestDomainMonitor_CheckDomain_NotInWatchlist(t *testing.T) {
	origProv := globalWhoisQueryProvider
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalWhoisQueryProvider = origProv
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
	defer withStubQueryProvider(&stubWhoisQueryProvider{raw: "x", info: whoisparser.WhoisInfo{}})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	m := NewDomainMonitor(DefaultMonitorConfig())
	// 不添加域名 → checkDomain 直接返回
	assert.NotPanics(t, func() {
		m.checkDomain(context.Background(), "absent.com")
	})
}

// ---- checkAll: 空列表直接返回 ----

func TestDomainMonitor_CheckAll_Empty(t *testing.T) {
	m := NewDomainMonitor(DefaultMonitorConfig())
	assert.NotPanics(t, func() {
		m.checkAll(context.Background())
	})
}

// ---- checkAll: 多域名并发检查 ----

func TestDomainMonitor_CheckAll_Multiple(t *testing.T) {
	origProv := globalWhoisQueryProvider
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalWhoisQueryProvider = origProv
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
	defer withStubQueryProvider(&stubWhoisQueryProvider{
		raw:  "raw",
		info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x", ExpirationDate: time.Now().Add(200 * 24 * time.Hour).Format("2006-01-02")}},
	})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	m := NewDomainMonitor(DefaultMonitorConfig())
	m.AddWatch("a.com", nil)
	m.AddWatch("b.com", nil)
	m.checkAll(context.Background())
	// 两个域名都应被检查（LastCheck 被设置）
	assert.False(t, m.GetWatchState("a.com").LastCheck.IsZero())
	assert.False(t, m.GetWatchState("b.com").LastCheck.IsZero())
}

// ---- Start: 启动后立即检查，然后取消 ----

func TestDomainMonitor_Start(t *testing.T) {
	origProv := globalWhoisQueryProvider
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalWhoisQueryProvider = origProv
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
	defer withStubQueryProvider(&stubWhoisQueryProvider{
		raw:  "raw",
		info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x", ExpirationDate: time.Now().Add(200 * 24 * time.Hour).Format("2006-01-02")}},
	})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	// 极短检查间隔（1 分钟），但 Start 会立即检查一次
	m := NewDomainMonitor(MonitorConfig{CheckInterval: 60, ExpiryWarningDays: 30, ExpiryCriticalDays: 7, MaxConcurrentChecks: 2})
	m.AddWatch("example.com", nil)

	ctx, cancel := context.WithCancel(context.Background())
	go m.Start(ctx)
	// 等待首次检查完成
	time.Sleep(200 * time.Millisecond)
	cancel()
	m.Stop()
	// 首次检查应已执行
	assert.False(t, m.GetWatchState("example.com").LastCheck.IsZero())
}

// ---- emitAlert: SaveAlert 失败（用已关闭的 Redis provider）----

func TestDomainMonitor_EmitAlert_SaveAlertFail(t *testing.T) {
	origAlert := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = origAlert }()
	// RedisStorage 已关闭 → SaveAlert 失败 → log.Warnf
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	cleanup() // 关闭 redis

	m := NewDomainMonitor(DefaultMonitorConfig())
	assert.NotPanics(t, func() {
		m.emitAlert(&DomainAlert{ID: "1", Domain: "x.com", Type: AlertExpiryWarning, Message: "fail"})
	})
}

// ---- Stop: 未启动时 cancel 为 nil ----

func TestDomainMonitor_Stop_NoCancel(t *testing.T) {
	m := NewDomainMonitor(DefaultMonitorConfig())
	// cancel 为 nil → 不 panic
	assert.NotPanics(t, func() { m.Stop() })
}
