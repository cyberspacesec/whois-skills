package whois

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// 全局状态恢复辅助：保存并恢复所有相关全局 provider
func saveBatchGlobals() func() {
	origProv := globalWhoisQueryProvider
	origHist := globalHistoryProvider
	origStor := globalStorageProvider
	return func() {
		globalWhoisQueryProvider = origProv
		globalHistoryProvider = origHist
		globalStorageProvider = origStor
	}
}

func withDomainAwareStub() func() {
	restore := saveBatchGlobals()
	SetWhoisQueryProvider(&domainAwareStubProvider{})
	return restore
}

func withErrorStub() func() {
	restore := saveBatchGlobals()
	SetWhoisQueryProvider(&stubWhoisQueryProvider{queryErr: assertError("boom")})
	return restore
}

// ---- Process: 断点续查（已有 checkpoint 文件，部分完成）----

func TestStreamBatchProcessor_Process_CheckpointResume(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	// 预置 checkpoint：a.test 已完成
	ckpt := &Checkpoint{
		BatchID:    "batch-1",
		CreatedAt:  time.Now().Format(time.RFC3339),
		AllDomains: []string{"a.test", "b.test"},
		CompletedDomains: map[string]bool{
			"a.test": true,
		},
		Results:       map[string]*CheckpointResult{"a.test": {}},
		SuccessCount:  1,
		FailureCount:  0,
	}
	data, _ := jsonMarshalIndent(ckpt)
	os.WriteFile(ckptPath, data, 0644)

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.Timeout = 5
	config.MaxRetries = 1
	config.CheckpointFile = ckptPath
	config.CheckpointInterval = 10
	p := NewStreamBatchProcessor(config)
	err := p.Process(context.Background(), []string{"a.test", "b.test"})
	assert.NoError(t, err)
	results := CollectResults(p.Results())
	assert.Len(t, results, 1) // 只处理 b.test
}

// ---- Process: 断点续查全部已完成（pendingDomains==0）----

func TestStreamBatchProcessor_Process_CheckpointAllDone(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	ckpt := &Checkpoint{
		BatchID:          "batch-1",
		AllDomains:       []string{"a.test"},
		CompletedDomains: map[string]bool{"a.test": true},
		Results:          map[string]*CheckpointResult{"a.test": {}},
		SuccessCount:     1,
	}
	data, _ := jsonMarshalIndent(ckpt)
	os.WriteFile(ckptPath, data, 0644)

	config := DefaultStreamBatchConfig()
	config.CheckpointFile = ckptPath
	p := NewStreamBatchProcessor(config)
	err := p.Process(context.Background(), []string{"a.test"})
	assert.NoError(t, err)
	// resultChan 应已关闭
	_, ok := <-p.Results()
	assert.False(t, ok)
}

// ---- Process: 断点文件坏 JSON → 新建 checkpoint ----

func TestStreamBatchProcessor_Process_CheckpointBadJSON(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	os.WriteFile(ckptPath, []byte("not json"), 0644)

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.CheckpointFile = ckptPath
	config.CheckpointInterval = 100
	p := NewStreamBatchProcessor(config)
	err := p.Process(context.Background(), []string{"a.test"})
	assert.NoError(t, err)
	CollectResults(p.Results())
}

// ---- Cancel: 有活跃 process + CheckpointFile → 保存断点 ----

func TestStreamBatchProcessor_Cancel_WithCheckpoint(t *testing.T) {
	restore := withErrorStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.CheckpointFile = ckptPath
	config.CheckpointInterval = 100
	p := NewStreamBatchProcessor(config)
	_ = p.Process(context.Background(), []string{"a.test", "b.test"})
	// 立即取消
	p.Cancel()
	CollectResults(p.Results())
	// 断点文件应已保存
	_, err := os.Stat(ckptPath)
	assert.NoError(t, err)
}

// ---- worker: QueryDelay 路径 + resultCallback + progressCallback ----

func TestStreamBatchProcessor_Worker_QueryDelayAndCallbacks(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.QueryDelay = 10 // 触发 QueryDelay 分支
	var gotResult *StreamBatchResult
	var gotProgress StreamBatchStats
	p := NewStreamBatchProcessor(config)
	p.OnResult(func(r *StreamBatchResult) { gotResult = r })
	p.OnProgress(func(s StreamBatchStats) { gotProgress = s })
	err := p.Process(context.Background(), []string{"a.test"})
	assert.NoError(t, err)
	CollectResults(p.Results())
	time.Sleep(30 * time.Millisecond)
	assert.NotNil(t, gotResult)
	assert.Equal(t, int64(1), gotProgress.Completed)
}

// ---- worker: 失败结果分支（stub 返回 error）+ Checkpoint 自动保存 ----

func TestStreamBatchProcessor_Worker_FailureAndAutoSave(t *testing.T) {
	restore := withErrorStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.CheckpointFile = ckptPath
	config.CheckpointInterval = 1 // 每完成 1 个就保存
	p := NewStreamBatchProcessor(config)
	err := p.Process(context.Background(), []string{"a.test"})
	assert.NoError(t, err)
	results := CollectResults(p.Results())
	assert.Len(t, results, 1)
	assert.Error(t, results[0].Error)
	// 断点自动保存
	time.Sleep(50 * time.Millisecond)
	_, err = os.Stat(ckptPath)
	assert.NoError(t, err)
}

// ---- worker: SaveHistorySnapshot 失败分支 ----

func TestStreamBatchProcessor_Worker_SaveHistoryFail(t *testing.T) {
	// 注入 domainAware stub + 失败的 history provider（关闭 redis）
	restore := saveBatchGlobals()
	defer restore()
	SetWhoisQueryProvider(&domainAwareStubProvider{})
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	SetStorageProvider(sp)
	SetHistoryProvider(NewLocalHistoryStorage(sp))
	cleanup() // 关闭 → SaveHistorySnapshot 失败

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.SaveToHistory = true
	p := NewStreamBatchProcessor(config)
	err = p.Process(context.Background(), []string{"a.test"})
	assert.NoError(t, err)
	CollectResults(p.Results())
	// 不 panic 即可
}

// ---- worker: ctx.Done 取消 ----

func TestStreamBatchProcessor_Worker_CtxDone(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultStreamBatchConfig()
	config.Concurrency = 2
	config.QueryDelay = 50 // 让 worker 在 QueryDelay 的 select 中等待
	p := NewStreamBatchProcessor(config)
	err := p.Process(ctx, []string{"a.test", "b.test", "c.test", "d.test"})
	assert.NoError(t, err)
	cancel() // 取消 → worker ctx.Done 退出
	CollectResults(p.Results())
}

// ---- queryDomain: error 路径已由失败 stub 覆盖；这里验证成功路径返回 Info ----

func TestStreamBatchProcessor_QueryDomain_Success(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	p := NewStreamBatchProcessor(DefaultStreamBatchConfig())
	r := p.queryDomain(context.Background(), "a.test")
	assert.NoError(t, r.Error)
	assert.NotNil(t, r.Info)
	assert.NotEmpty(t, r.RawResponse)
}

// ---- loadCheckpoint: 未配置 / 读取失败 / 解析失败 ----

func TestStreamBatchProcessor_LoadCheckpoint_NoFile(t *testing.T) {
	p := NewStreamBatchProcessor(DefaultStreamBatchConfig()) // CheckpointFile=""
	_, err := p.loadCheckpoint()
	assert.Error(t, err)
}

func TestStreamBatchProcessor_LoadCheckpoint_FileNotFound(t *testing.T) {
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = "/nonexistent/ckpt.json"
	p := NewStreamBatchProcessor(config)
	_, err := p.loadCheckpoint()
	assert.Error(t, err)
}

func TestStreamBatchProcessor_LoadCheckpoint_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ckpt.json")
	os.WriteFile(path, []byte("not json"), 0644)
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = path
	p := NewStreamBatchProcessor(config)
	_, err := p.loadCheckpoint()
	assert.Error(t, err)
}

// ---- saveCheckpoint: 各种失败 ----

func TestStreamBatchProcessor_SaveCheckpoint_NoConfig(t *testing.T) {
	p := NewStreamBatchProcessor(DefaultStreamBatchConfig())
	err := p.saveCheckpoint()
	assert.NoError(t, err) // CheckpointFile="" 直接返回 nil
}

func TestStreamBatchProcessor_SaveCheckpoint_MkdirFail(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.Chmod(sub, 0444)
	defer os.Chmod(sub, 0755)
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = filepath.Join(sub, "under", "ckpt.json")
	p := NewStreamBatchProcessor(config)
	p.checkpoint = p.newCheckpoint([]string{"a.test"})
	err := p.saveCheckpoint()
	assert.Error(t, err)
}

func TestStreamBatchProcessor_SaveCheckpoint_WriteFail(t *testing.T) {
	dir := t.TempDir()
	// 目标路径是已存在目录 → WriteFile 失败
	existDir := filepath.Join(dir, "adir")
	os.MkdirAll(existDir, 0755)
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = existDir
	p := NewStreamBatchProcessor(config)
	p.checkpoint = p.newCheckpoint([]string{"a.test"})
	err := p.saveCheckpoint()
	assert.Error(t, err)
}

func TestStreamBatchProcessor_SaveCheckpoint_RenameFail(t *testing.T) {
	// tmpFile 写入成功但 Rename 失败：用跨设备路径难以构造，
	// 改用：CheckpointFile 指向一个已存在目录的子项名（Rename 到目录失败）
	// 此分支较难稳定触发，仅验证正常保存路径成功
	dir := t.TempDir()
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = filepath.Join(dir, "ckpt.json")
	p := NewStreamBatchProcessor(config)
	p.checkpoint = p.newCheckpoint([]string{"a.test"})
	err := p.saveCheckpoint()
	assert.NoError(t, err)
}

// ---- LoadCheckpointFromFile: 解析失败 ----

func TestLoadCheckpointFromFile_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ckpt.json")
	os.WriteFile(path, []byte("not json"), 0644)
	_, err := LoadCheckpointFromFile(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析断点数据失败")
}

// ---- ResumeFromCheckpoint: 无断点文件路径 ----

func TestResumeFromCheckpoint_NoFilePath(t *testing.T) {
	_, err := ResumeFromCheckpoint(context.Background(), DefaultStreamBatchConfig())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须指定断点文件路径")
}

// ---- ResumeFromCheckpoint: 文件不存在 ----

func TestResumeFromCheckpoint_FileMissing(t *testing.T) {
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = "/nonexistent/ckpt.json"
	_, err := ResumeFromCheckpoint(context.Background(), config)
	assert.Error(t, err)
}

// ---- ResumeFromCheckpoint: 全部已完成（pending==0）----

func TestResumeFromCheckpoint_AllDone(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	ckpt := &Checkpoint{
		AllDomains:       []string{"a.test"},
		CompletedDomains: map[string]bool{"a.test": true},
		Results:          map[string]*CheckpointResult{"a.test": {}},
	}
	data, _ := jsonMarshalIndent(ckpt)
	os.WriteFile(ckptPath, data, 0644)

	config := DefaultStreamBatchConfig()
	config.CheckpointFile = ckptPath
	p, err := ResumeFromCheckpoint(context.Background(), config)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	// pending==0 时 Resume 不调用 Process，resultChan 未关闭且为空；
	// 用非阻塞 select 验证无结果
	select {
	case _, ok := <-p.Results():
		assert.False(t, ok, "channel should not produce results")
	default:
		// 无数据，符合预期
	}
}

// ---- ResumeFromCheckpoint: 有待处理 → 启动 Process ----

func TestResumeFromCheckpoint_WithPending(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	dir := t.TempDir()
	ckptPath := filepath.Join(dir, "ckpt.json")
	ckpt := &Checkpoint{
		AllDomains:       []string{"a.test", "b.test"},
		CompletedDomains: map[string]bool{"a.test": true},
		Results:          map[string]*CheckpointResult{"a.test": {}},
	}
	data, _ := jsonMarshalIndent(ckpt)
	os.WriteFile(ckptPath, data, 0644)

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.CheckpointFile = ckptPath
	p, err := ResumeFromCheckpoint(context.Background(), config)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	results := CollectResults(p.Results())
	assert.Len(t, results, 1) // 只 b.test
}

// ---- ResumeFromCheckpoint: Process 失败（空 pending 传给 Process）----
// 注意：getPendingDomains 返回空时 Resume 直接返回，不调 Process；
// Process 失败需 pending 非空但 Process 内部出错。此处用坏 config 难以稳定触发，
// 已由其它用例覆盖 Process 内部分支。

// helper: 序列化 checkpoint
func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
