package whois

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== batch.go worker ctx 取消分支 ====================

// TestStreamBatchWorker_QueryDelayCtxCancelled QueryDelay>0 且 ctx 已取消 →
// worker 在 QueryDelay 等待 select 命中 ctx.Done 返回（line 340-341）。
func TestStreamBatchWorker_QueryDelayCtxCancelled(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.QueryDelay = 5000 // 长延迟，确保 worker 阻塞在等待
	p := NewStreamBatchProcessor(config)

	ctx, cancel := context.WithCancel(context.Background())
	domainChan := make(chan string, 1)
	domainChan <- "a.test"
	close(domainChan)

	// 启动 worker 后立即取消 ctx，让 QueryDelay 等待 select 命中 ctx.Done
	done := make(chan struct{})
	go func() {
		p.worker(ctx, domainChan)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond) // 等 worker 进入 QueryDelay 等待
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker 未在 ctx 取消后退出")
	}
	// worker 提前退出，未产出结果
	assert.Equal(t, int64(0), p.completed)
}

// TestStreamBatchWorker_SendResultCtxCancelled worker 在发送结果时 ctx 取消
// → resultChan <- result select 命中 ctx.Done 返回（line 351-352）。
// 通过不消费 resultChan 使其满，再取消 ctx。
func TestStreamBatchWorker_SendResultCtxCancelled(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()
	defer registerLocalWhoisServer("test", "whois.verisign-grs.com")()

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.QueryDelay = 0
	p := NewStreamBatchProcessor(config)
	// resultChan 缓冲 = Concurrency*2 = 2。填满 2 个让发送阻塞
	p.resultChan <- &StreamBatchResult{Domain: "filler1"}
	p.resultChan <- &StreamBatchResult{Domain: "filler2"}

	ctx, cancel := context.WithCancel(context.Background())
	domainChan := make(chan string, 1)
	domainChan <- "a.test"
	close(domainChan)

	done := make(chan struct{})
	go func() {
		p.worker(ctx, domainChan)
		close(done)
	}()
	time.Sleep(80 * time.Millisecond) // 等 worker 查询完成并阻塞在 resultChan <-
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker 未在发送结果 ctx 取消后退出")
	}
}

// ==================== batch.go saveCheckpoint WriteFile 失败 ====================

// TestStreamBatchProcessor_SaveCheckpoint_WriteFailReadOnly tmpFile 落在只读目录 → WriteFile 失败。
func TestStreamBatchProcessor_SaveCheckpoint_WriteFailReadOnly(t *testing.T) {
	dir := t.TempDir()
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = filepath.Join(dir, "sub", "ckpt.json")
	p := NewStreamBatchProcessor(config)
	p.checkpoint = p.newCheckpoint([]string{"a.com"})

	// 把 dir/sub 设为只读目录，使 WriteFile(dir/sub/ckpt.json.tmp) 失败
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0555)
	defer os.Chmod(subDir, 0755) // 恢复以便清理

	err := p.saveCheckpoint()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "写入断点临时文件失败")
}

// TestStreamBatchProcessor_SaveCheckpoint_EmptyFile 无 CheckpointFile → 直接返回 nil。
func TestStreamBatchProcessor_SaveCheckpoint_EmptyFile(t *testing.T) {
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = ""
	p := NewStreamBatchProcessor(config)
	assert.NoError(t, p.saveCheckpoint())
}

// ==================== batch.go ResumeFromCheckpoint Process 失败 ====================

// TestResumeFromCheckpoint_ProcessFail pending 非空 + 空 domains 传入 Process
// 实际上 ResumeFromCheckpoint 用 getPendingDomains。这里用不存在的 checkpoint 文件触发
// LoadCheckpointFromFile 失败（已覆盖）。改测 pending 为空直接返回（line 544-546）。
func TestResumeFromCheckpoint_PendingEmpty(t *testing.T) {
	restore := withDomainAwareStub()
	defer restore()

	dir := t.TempDir()
	// 构造一个全部已完成的 checkpoint
	ckpt := &Checkpoint{
		AllDomains:       []string{"a.test"},
		CompletedDomains: map[string]bool{"a.test": true}, // 全部完成 → pending 为空
		SuccessCount:     1,
	}
	data, _ := jsonMarshalIndent(ckpt)
	ckptPath := filepath.Join(dir, "ckpt.json")
	os.WriteFile(ckptPath, data, 0644)

	config := DefaultStreamBatchConfig()
	config.CheckpointFile = ckptPath
	p, err := ResumeFromCheckpoint(context.Background(), config)
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

// ==================== monitor.go checkAll ctx 取消 ====================

// TestDomainMonitor_checkAll_CtxCancelled checkAll 在迭代域名时 ctx 已取消 → return。
func TestDomainMonitor_checkAll_CtxCancelled(t *testing.T) {
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()
	defer withStubQueryProvider(&availStubProvider{
		info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}},
	})()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)

	m := NewDomainMonitor(DefaultMonitorConfig())
	m.AddWatch("a.com", nil)
	m.AddWatch("b.com", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 已取消 → checkAll 首个域名 select 命中 ctx.Done
	assert.NotPanics(t, func() { m.checkAll(ctx) })
}

// ==================== monitor.go checkDomain SaveWatchState 失败 ====================

// failingMonitorStateProvider SaveWatchState 总是报错。
type failingMonitorStateProvider struct{}

func (failingMonitorStateProvider) SaveWatchState(ctx context.Context, state *DomainWatchState) error {
	return errors.New("persist failed")
}
func (failingMonitorStateProvider) LoadWatchStates(ctx context.Context) (map[string]*DomainWatchState, error) {
	return nil, nil
}
func (failingMonitorStateProvider) DeleteWatchState(ctx context.Context, domain string) error {
	return nil
}
func (failingMonitorStateProvider) Close() error { return nil }

// TestDomainMonitor_checkDomain_SaveWatchStateFail SaveWatchState 失败 → logrus.Warn 分支。
func TestDomainMonitor_checkDomain_SaveWatchStateFail(t *testing.T) {
	origAlert := globalAlertStorageProvider
	origMonitor := globalMonitorStateProvider
	defer func() {
		globalAlertStorageProvider = origAlert
		globalMonitorStateProvider = origMonitor
	}()
	defer withStubQueryProvider(&availStubProvider{
		info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "a.com"}},
	})()

	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalMonitorStateProvider = failingMonitorStateProvider{} // 持久化失败

	m := NewDomainMonitor(DefaultMonitorConfig())
	m.AddWatch("a.com", nil)
	// 不应 panic，仅 warn
	assert.NotPanics(t, func() { m.checkDomain(context.Background(), "a.com") })
}
