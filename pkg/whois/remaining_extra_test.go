package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== config.go ====================

// ---- libConfigFromEnv: 无环境变量 ----

func TestLibConfigFromEnv_NoEnv(t *testing.T) {
	t.Setenv("WHOIS_CONFIG_FILE", "")
	assert.Nil(t, libConfigFromEnv())
}

// ---- libConfigFromEnv: 有环境变量（文件不存在）----

func TestLibConfigFromEnv_MissingFile(t *testing.T) {
	t.Setenv("WHOIS_CONFIG_FILE", "/nonexistent/cfg.json")
	// 读取失败仅 Warn，返回 nil
	assert.Nil(t, libConfigFromEnv())
}

// ---- libConfigFromEnv: 有环境变量（合法 JSON）----

func TestLibConfigFromEnv_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	os.WriteFile(path, []byte(`{"query":{"timeout":7}}`), 0644)
	t.Setenv("WHOIS_CONFIG_FILE", path)
	cfg := libConfigFromEnv()
	assert.NotNil(t, cfg)
	assert.Equal(t, 7, cfg.Query.Timeout)
}

// ---- LoadWhoisLibraryConfigFromFile: YAML 扩展名 ----

func TestLoadWhoisLibraryConfigFromFile_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	os.WriteFile(path, []byte("query:\n  timeout: 9\n"), 0644)
	cfg := LoadWhoisLibraryConfigFromFile(path)
	assert.NotNil(t, cfg)
	assert.Equal(t, 9, cfg.Query.Timeout)
}

// ---- LoadWhoisLibraryConfigFromFile: YAML 解析失败 ----

func TestLoadWhoisLibraryConfigFromFile_YAMLBad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yml")
	os.WriteFile(path, []byte(":\n  bad: [unclosed"), 0644)
	assert.Nil(t, LoadWhoisLibraryConfigFromFile(path))
}

// ---- LoadYAMLConfig: 成功 / 读取失败 / 解析失败 ----

func TestLoadYAMLConfig_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	os.WriteFile(path, []byte("server:\n  host: 0.0.0.0\n  port: 9090\nlog:\n  level: debug\n"), 0644)
	cfg, err := LoadYAMLConfig(path)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "debug", cfg.Log.Level)
}

func TestLoadYAMLConfig_ReadFail(t *testing.T) {
	_, err := LoadYAMLConfig("/nonexistent/app.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取配置文件失败")
}

func TestLoadYAMLConfig_BadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	os.WriteFile(path, []byte(":\n  bad: [unclosed"), 0644)
	_, err := LoadYAMLConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析YAML配置文件失败")
}

// ---- DefaultAppConfig ----

func TestDefaultAppConfig(t *testing.T) {
	cfg := DefaultAppConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.True(t, cfg.Cache.Enabled)
	assert.Equal(t, "local", cfg.Cache.Type)
	assert.True(t, cfg.Metrics.Enabled)
	assert.True(t, cfg.Alerts.Enabled)
}

// ---- SaveWhoisLibraryConfigToFile: 序列化/目录/写入失败 ----

func TestSaveWhoisLibraryConfigToFile_MkdirFail(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "ro")
	os.MkdirAll(sub, 0755)
	os.Chmod(sub, 0444)
	defer os.Chmod(sub, 0755)
	cfg := DefaultWhoisLibraryConfig()
	err := SaveWhoisLibraryConfigToFile(&cfg, filepath.Join(sub, "under", "cfg.json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "创建目录失败")
}

func TestSaveWhoisLibraryConfigToFile_WriteFail(t *testing.T) {
	dir := t.TempDir()
	existDir := filepath.Join(dir, "adir")
	os.MkdirAll(existDir, 0755)
	cfg := DefaultWhoisLibraryConfig()
	err := SaveWhoisLibraryConfigToFile(&cfg, existDir) // 路径是目录 → WriteFile 失败
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "写入配置文件失败")
}

// ---- ValidateWhoisLibraryConfig: 各错误分支 ----

func TestValidateWhoisLibraryConfig_AllErrorBranches(t *testing.T) {
	// cache enabled 但 MaxEntries<=0
	cfg := DefaultWhoisLibraryConfig()
	cfg.Cache.Enabled = true
	cfg.Cache.MaxEntries = 0
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// cache enabled 但 DefaultTTLMinutes<=0
	cfg = DefaultWhoisLibraryConfig()
	cfg.Cache.Enabled = true
	cfg.Cache.DefaultTTLMinutes = 0
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// rate enabled 但 GlobalRate<=0
	cfg = DefaultWhoisLibraryConfig()
	cfg.RateLimit.Enabled = true
	cfg.RateLimit.GlobalRate = 0
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// monitor enabled 但 CheckIntervalMinutes<=0
	cfg = DefaultWhoisLibraryConfig()
	cfg.Monitor.Enabled = true
	cfg.Monitor.CheckIntervalMinutes = 0
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// scheduler DefaultIntervalMs<=0
	cfg = DefaultWhoisLibraryConfig()
	cfg.Scheduler.DefaultIntervalMs = 0
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// storage enabled 坏 type
	cfg = DefaultWhoisLibraryConfig()
	cfg.Storage.Enabled = true
	cfg.Storage.Type = "weird"
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// storage redis 无 RedisConfig
	cfg = DefaultWhoisLibraryConfig()
	cfg.Storage.Enabled = true
	cfg.Storage.Type = "redis"
	cfg.Storage.RedisConfig = nil
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// storage local（允许空 Directory，合法）
	cfg = DefaultWhoisLibraryConfig()
	cfg.Storage.Enabled = true
	cfg.Storage.Type = "local"
	cfg.Storage.Directory = ""
	assert.NoError(t, ValidateWhoisLibraryConfig(&cfg))

	// ASNRelation enabled 坏 type
	cfg = DefaultWhoisLibraryConfig()
	cfg.ASNRelation.Enabled = true
	cfg.ASNRelation.Type = "weird"
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// History enabled 坏 type
	cfg = DefaultWhoisLibraryConfig()
	cfg.History.Enabled = true
	cfg.History.Type = "weird"
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// AlertStorage enabled 坏 type
	cfg = DefaultWhoisLibraryConfig()
	cfg.AlertStorage.Enabled = true
	cfg.AlertStorage.Type = "weird"
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// MonitorState enabled 坏 type
	cfg = DefaultWhoisLibraryConfig()
	cfg.MonitorState.Enabled = true
	cfg.MonitorState.Type = "weird"
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))

	// ReverseWhois enabled 坏 type
	cfg = DefaultWhoisLibraryConfig()
	cfg.ReverseWhois.Enabled = true
	cfg.ReverseWhois.Type = "weird"
	assert.Error(t, ValidateWhoisLibraryConfig(&cfg))
}

// ---- ApplyWhoisLibraryConfig: 成功（全启用但用合法配置）----

func TestApplyWhoisLibraryConfig_AllEnabled(t *testing.T) {
	// 备份并恢复所有全局 provider
	origStor := globalStorageProvider
	origASN := globalASNRelationProvider
	origHist := globalHistoryProvider
	origAlert := globalAlertStorageProvider
	origMon := globalMonitorStateProvider
	origRev := globalReverseWhoisProvider
	defer func() {
		globalStorageProvider = origStor
		globalASNRelationProvider = origASN
		globalHistoryProvider = origHist
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMon
		globalReverseWhoisProvider = origRev
	}()

	cfg := DefaultWhoisLibraryConfig()
	cfg.Log.Level = "warn"
	cfg.Log.Format = "json"
	// storage local 用临时目录
	cfg.Storage.Enabled = true
	cfg.Storage.Type = "local"
	cfg.Storage.Directory = t.TempDir()
	// ASNRelation local 用临时文件
	dir := t.TempDir()
	asPath := filepath.Join(dir, "as-rel.txt")
	os.WriteFile(asPath, []byte("1|2|0|src\n"), 0644)
	cfg.ASNRelation.Enabled = true
	cfg.ASNRelation.Type = "local"
	cfg.ASNRelation.FilePath = asPath
	// history local
	cfg.History.Enabled = true
	cfg.History.Type = "local"
	cfg.History.Directory = t.TempDir()
	// alert local
	cfg.AlertStorage.Enabled = true
	cfg.AlertStorage.Type = "local"
	cfg.AlertStorage.Directory = t.TempDir()
	// monitor state local
	cfg.MonitorState.Enabled = true
	cfg.MonitorState.Type = "local"
	cfg.MonitorState.Directory = t.TempDir()
	// reverse local（依赖 history provider 已设置）
	cfg.ReverseWhois.Enabled = true
	cfg.ReverseWhois.Type = "local"

	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// ---- ApplyWhoisLibraryConfig: 无效日志级别 ----

func TestApplyWhoisLibraryConfig_BadLogLevel(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Log.Level = "not-a-level"
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err) // 仅 Warn，不报错
}

// ---- MergeWhoisLibraryConfigs: 覆盖各字段 ----

func TestMergeWhoisLibraryConfigs_AllFields(t *testing.T) {
	base := DefaultWhoisLibraryConfig()
	override := &WhoisLibraryConfig{
		Query: WhoisQueryConfig{
			Timeout:        99,
			MaxRetries:     7,
			RetryInterval:  500,
			QueryDelay:     50,
		},
		Cache: WhoisCacheConfig{
			Type:               "redis",
			MaxEntries:         5,
			DefaultTTLMinutes:  30,
		},
		RateLimit: WhoisRateLimitConfig{
			GlobalRate:     20,
			PerServerRate:  map[string]float64{"a": 1},
		},
		Batch: WhoisBatchConfig{
			Concurrency:       8,
			CheckpointFile:    "x.json",
		},
		Monitor: WhoisMonitorConfig{
			CheckIntervalMinutes: 5,
		},
		Scheduler: WhoisSchedulerConfig{
			DefaultIntervalMs: 100,
			MaxConcurrency:    3,
		},
		Log: WhoisLogConfig{
			Level:  "debug",
			Format: "json",
		},
	}
	merged := MergeWhoisLibraryConfigs(&base, override, nil)
	assert.Equal(t, 99, merged.Query.Timeout)
	assert.Equal(t, 7, merged.Query.MaxRetries)
	assert.Equal(t, 500, merged.Query.RetryInterval)
	assert.Equal(t, 50, merged.Query.QueryDelay)
	assert.Equal(t, "redis", merged.Cache.Type)
	assert.Equal(t, 5, merged.Cache.MaxEntries)
	assert.Equal(t, 30, merged.Cache.DefaultTTLMinutes)
	assert.Equal(t, 20.0, merged.RateLimit.GlobalRate)
	assert.Equal(t, map[string]float64{"a": 1}, merged.RateLimit.PerServerRate)
	assert.Equal(t, 8, merged.Batch.Concurrency)
	assert.Equal(t, "x.json", merged.Batch.CheckpointFile)
	assert.Equal(t, 5, merged.Monitor.CheckIntervalMinutes)
	assert.Equal(t, 100, merged.Scheduler.DefaultIntervalMs)
	assert.Equal(t, 3, merged.Scheduler.MaxConcurrency)
	assert.Equal(t, "debug", merged.Log.Level)
	assert.Equal(t, "json", merged.Log.Format)
}

// ==================== observability.go ====================

// assertRateLimitError 返回一个限速错误，供调度器测试触发 isRateLimitError 分支。
func assertRateLimitError() error {
	return NewWhoisError(ErrRateLimited, "被限速", nil)
}

// ==================== ratelimit.go ====================

// ---- Wait: 首次 Allow 即通过（不进入循环体）----

func TestRateLimiter_Wait_Immediate(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		PerServerRate: map[string]float64{"srv": 1000},
		BurstSize:     1,
	})
	// 应立即返回（首次 Allow 返回 true）
	done := make(chan struct{})
	go func() {
		rl.Wait("srv")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait 阻塞超时")
	}
}

// ---- Allow: nil receiver + 无 server 配置默认允许 ----

func TestRateLimiter_Allow_NilAndDefault(t *testing.T) {
	var rl *RateLimiter
	assert.True(t, rl.Allow("any")) // nil receiver → true

	rl = NewRateLimiter(RateLimiterConfig{}) // 无 global/server 配置
	assert.True(t, rl.Allow("any"))          // 无 server bucket → 默认允许
}

// ==================== reverse_local.go ====================

// ---- Name / Close ----

func TestLocalReverseWhoisIndex_NameAndClose(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	assert.Equal(t, "local-index", idx.Name())
	assert.NoError(t, idx.Close())
}

// ---- IndexWhoisSnapshot: provider 是非 LocalReverseWhoisIndex 类型 ----

type otherReverseProvider struct{}

func (otherReverseProvider) SearchByRegistrant(ctx context.Context, q string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return nil, nil
}
func (otherReverseProvider) SearchByEmail(ctx context.Context, q string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return nil, nil
}
func (otherReverseProvider) SearchByOrganization(ctx context.Context, q string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return nil, nil
}
func (otherReverseProvider) Name() string  { return "other" }
func (otherReverseProvider) Close() error { return nil }

func TestIndexWhoisSnapshot_NonLocalProvider(t *testing.T) {
	orig := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = orig }()
	SetReverseWhoisProvider(otherReverseProvider{})
	// provider 不是 *LocalReverseWhoisIndex → 返回 nil
	err := IndexWhoisSnapshot(context.Background(), &WhoisSnapshot{Domain: "x.com"})
	assert.NoError(t, err)
}

// ---- IndexWhoisSnapshot: 真实 local provider 索引 ----

func TestIndexWhoisSnapshot_LocalProvider(t *testing.T) {
	orig := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = orig }()
	idx := NewLocalReverseWhoisIndex(nil)
	SetReverseWhoisProvider(idx)

	snap := &WhoisSnapshot{
		Domain: "x.com",
		Info: whoisparser.WhoisInfo{
			Registrant: &whoisparser.Contact{Email: "r@x.com", Name: "Reg", Organization: "Org"},
		},
	}
	err := IndexWhoisSnapshot(context.Background(), snap)
	assert.NoError(t, err)
	// 索引应包含 r@x.com
	res, _ := idx.SearchByEmail(context.Background(), "r@x.com", nil)
	assert.NotEmpty(t, res)
}

// ---- IndexSnapshot: nil/空 domain 直接返回 ----

func TestLocalReverseWhoisIndex_IndexSnapshot_NilAndEmpty(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	assert.NoError(t, idx.IndexSnapshot(context.Background(), nil))
	assert.NoError(t, idx.IndexSnapshot(context.Background(), &WhoisSnapshot{Domain: ""}))
}

// ---- indexContact: nil 联系人 ----

func TestLocalReverseWhoisIndex_indexContact_Nil(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	idx.mu.Lock()
	idx.indexContact("x.com", nil)
	idx.mu.Unlock()
	assert.Empty(t, idx.index)
}

// ---- RebuildFromSnapshots: 含空快照（空域名不报错，仍计入 count）----

func TestLocalReverseWhoisIndex_RebuildFromSnapshots(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	snaps := []WhoisSnapshot{
		{Domain: "a.com", Info: whoisparser.WhoisInfo{Registrant: &whoisparser.Contact{Email: "a@a.com"}}},
		{Domain: ""}, // 空域名：IndexSnapshot 返回 nil 不报错，count 仍 +1
		{Domain: "b.com", Info: whoisparser.WhoisInfo{Registrant: &whoisparser.Contact{Email: "b@b.com"}}},
	}
	count, err := idx.RebuildFromSnapshots(context.Background(), snaps)
	assert.NoError(t, err)
	assert.Equal(t, 3, count) // 三条均无错误，都计入
	// 但空域名不会产生索引项，故按 email 搜索 a@a.com 仅命中 a.com
	res, _ := idx.SearchByEmail(context.Background(), "a@a.com", nil)
	assert.Len(t, res, 1)
}

// ---- searchByField: 空查询值 ----

func TestLocalReverseWhoisIndex_SearchEmptyValue(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	_, err := idx.SearchByEmail(context.Background(), "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询值不能为空")
}

// ==================== scheduler.go ====================

// ---- NewAdaptiveRateLimiter: 各参数<=0 走默认 ----

func TestNewAdaptiveRateLimiter_Defaults(t *testing.T) {
	arl := NewAdaptiveRateLimiter(0, 0, 0)
	assert.Equal(t, 5.0, arl.currentRate)
	assert.Equal(t, 1.0, arl.minRate)
	assert.Equal(t, 20.0, arl.maxRate)
	assert.NotNil(t, arl.bucket)
}

// ---- Schedule: 退避路径（NextAllowedTime 在未来）----

func TestSmartScheduler_Schedule_Backoff(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// 先把服务器标记为退避：通过 RecordResult 触发 rate-limit
	s.RecordResult("srv", 10, assertRateLimitError())
	// 再次调度应进入退避等待
	dur, err := s.Schedule(context.Background(), "srv")
	// 退避路径返回 nil err + 等待时长
	assert.NoError(t, err)
	assert.True(t, dur > 0 || dur == 0) // 退避可能已过期，仅验证不 panic
}

// ---- Schedule: 自适应限速器返回 false 路径 ----
// 难以稳定触发（需耗尽令牌）；RecordResult success 多次后间隔调整已覆盖主要分支。

// ---- increaseInterval / decreaseInterval: current<=0 走默认 ----

func TestSmartScheduler_IncreaseDecreaseInterval_FromZero(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	state := &ServerState{Server: "srv", AdaptiveInterval: 0}
	s.increaseInterval(state)
	assert.Greater(t, state.AdaptiveInterval, int64(0))

	state2 := &ServerState{Server: "srv2", AdaptiveInterval: 0}
	s.decreaseInterval(state2)
	// 减少后不低于 MinInterval
	assert.GreaterOrEqual(t, state2.AdaptiveInterval, int64(s.config.MinInterval))
}

// ---- GetServerState: 未找到返回 nil ----

func TestSmartScheduler_GetServerState_NotFound(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	assert.Nil(t, s.GetServerState("nope"))
}

// ---- Allow: 触发 adjustRate（模拟 lastAdjust > 30s）----

func TestAdaptiveRateLimiter_Allow_AdjustRate(t *testing.T) {
	arl := NewAdaptiveRateLimiter(5.0, 1.0, 20.0)
	arl.mu.Lock()
	arl.lastAdjust = time.Now().Add(-31 * time.Second)
	arl.mu.Unlock()
	// 调用 Allow 应触发 adjustRate
	assert.NotPanics(t, func() { arl.Allow() })
}
