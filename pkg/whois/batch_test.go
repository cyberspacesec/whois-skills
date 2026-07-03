package whois

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewStreamBatchProcessor(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)

	if processor == nil {
		t.Fatal("NewStreamBatchProcessor() returned nil")
	}
	if processor.config.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want 5", processor.config.Concurrency)
	}
}

func TestNewStreamBatchProcessor_Defaults(t *testing.T) {
	// 测试零值配置会使用默认值
	config := StreamBatchConfig{}
	processor := NewStreamBatchProcessor(config)

	if processor.config.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want 5", processor.config.Concurrency)
	}
	if processor.config.Timeout != 10 {
		t.Errorf("Timeout = %d, want 10", processor.config.Timeout)
	}
	if processor.config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", processor.config.MaxRetries)
	}
}

func TestStreamBatchProcessor_Process_EmptyDomains(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)

	err := processor.Process(context.Background(), []string{})
	if err == nil {
		t.Error("Expected error for empty domain list")
	}
}

func TestStreamBatchProcessor_GetStats(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)
	processor.totalTasks = 10
	processor.startTime = time.Now()

	stats := processor.GetStats()
	if stats.TotalTasks != 10 {
		t.Errorf("TotalTasks = %d, want 10", stats.TotalTasks)
	}
	if stats.Completed != 0 {
		t.Errorf("Completed = %d, want 0", stats.Completed)
	}
}

func TestStreamBatchProcessor_Cancel(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)

	// Cancel should not panic even without active process
	processor.Cancel()
}

func TestStreamBatchProcessor_OnProgress(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)

	called := false
	processor.OnProgress(func(stats StreamBatchStats) {
		called = true
	})

	if processor.progressCallback == nil {
		t.Error("Progress callback should be set")
	}
	// Call it manually to verify
	processor.progressCallback(StreamBatchStats{})
	if !called {
		t.Error("Progress callback should have been called")
	}
}

func TestStreamBatchProcessor_OnResult(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)

	processor.OnResult(func(result *StreamBatchResult) {
	})

	if processor.resultCallback == nil {
		t.Error("Result callback should be set")
	}
}

func TestStreamBatchProcessor_Results(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)

	ch := processor.Results()
	if ch == nil {
		t.Error("Results channel should not be nil")
	}
}

func TestCheckpoint_New(t *testing.T) {
	domains := []string{"a.com", "b.com", "c.com"}
	processor := NewStreamBatchProcessor(DefaultStreamBatchConfig())
	checkpoint := processor.newCheckpoint(domains)

	if checkpoint.BatchID == "" {
		t.Error("BatchID should not be empty")
	}
	if checkpoint.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if len(checkpoint.AllDomains) != 3 {
		t.Errorf("AllDomains count = %d, want 3", len(checkpoint.AllDomains))
	}
	if len(checkpoint.CompletedDomains) != 0 {
		t.Errorf("CompletedDomains should be empty initially")
	}
	if len(checkpoint.Results) != 0 {
		t.Errorf("Results should be empty initially")
	}
}

func TestCheckpoint_GetPendingDomains(t *testing.T) {
	processor := NewStreamBatchProcessor(DefaultStreamBatchConfig())
	processor.checkpoint = &Checkpoint{
		AllDomains:       []string{"a.com", "b.com", "c.com", "d.com"},
		CompletedDomains: map[string]bool{"a.com": true, "c.com": true},
		Results:          make(map[string]*CheckpointResult),
	}

	pending := processor.getPendingDomains([]string{"a.com", "b.com", "c.com", "d.com"})

	if len(pending) != 2 {
		t.Errorf("Pending count = %d, want 2", len(pending))
	}

	// Check that b.com and d.com are in pending
	foundB, foundD := false, false
	for _, d := range pending {
		if d == "b.com" {
			foundB = true
		}
		if d == "d.com" {
			foundD = true
		}
	}
	if !foundB || !foundD {
		t.Errorf("Expected b.com and d.com in pending, got %v", pending)
	}
}

func TestCheckpoint_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultStreamBatchConfig()
	config.CheckpointFile = checkpointFile
	processor := NewStreamBatchProcessor(config)

	// Create and save checkpoint
	processor.checkpoint = &Checkpoint{
		BatchID:          "test-batch",
		CreatedAt:        "2024-01-01T00:00:00Z",
		AllDomains:       []string{"a.com", "b.com", "c.com"},
		CompletedDomains: map[string]bool{"a.com": true},
		Results: map[string]*CheckpointResult{
			"a.com": {RawResponse: "whois data for a.com", Latency: 100, RetryCount: 0},
		},
		TotalTasks:   3,
		SuccessCount: 1,
		FailureCount: 0,
	}

	err := processor.saveCheckpoint()
	if err != nil {
		t.Fatalf("saveCheckpoint() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Fatal("Checkpoint file should exist")
	}

	// Load checkpoint
	loaded, err := processor.loadCheckpoint()
	if err != nil {
		t.Fatalf("loadCheckpoint() error = %v", err)
	}

	if loaded.BatchID != "test-batch" {
		t.Errorf("BatchID = %s, want test-batch", loaded.BatchID)
	}
	if len(loaded.AllDomains) != 3 {
		t.Errorf("AllDomains count = %d, want 3", len(loaded.AllDomains))
	}
	if !loaded.CompletedDomains["a.com"] {
		t.Error("a.com should be marked as completed")
	}
	if loaded.CompletedDomains["b.com"] {
		t.Error("b.com should not be marked as completed")
	}
	if loaded.Results["a.com"].Latency != 100 {
		t.Errorf("a.com latency = %d, want 100", loaded.Results["a.com"].Latency)
	}
}

func TestCheckpoint_SaveNoFile(t *testing.T) {
	config := DefaultStreamBatchConfig()
	// No checkpoint file configured
	processor := NewStreamBatchProcessor(config)

	err := processor.saveCheckpoint()
	if err != nil {
		t.Errorf("saveCheckpoint() with no file should return nil, got %v", err)
	}
}

func TestCheckpoint_LoadNoFile(t *testing.T) {
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = "/nonexistent/path/checkpoint.json"
	processor := NewStreamBatchProcessor(config)

	_, err := processor.loadCheckpoint()
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadCheckpointFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "checkpoint.json")

	checkpoint := &Checkpoint{
		BatchID:          "test-load",
		CreatedAt:        "2024-01-01T00:00:00Z",
		AllDomains:       []string{"x.com", "y.com"},
		CompletedDomains: map[string]bool{"x.com": true},
		Results:          make(map[string]*CheckpointResult),
	}

	data, _ := json.MarshalIndent(checkpoint, "", "  ")
	os.WriteFile(checkpointFile, data, 0644)

	loaded, err := LoadCheckpointFromFile(checkpointFile)
	if err != nil {
		t.Fatalf("LoadCheckpointFromFile() error = %v", err)
	}
	if loaded.BatchID != "test-load" {
		t.Errorf("BatchID = %s, want test-load", loaded.BatchID)
	}
}

func TestLoadCheckpointFromFile_NotFound(t *testing.T) {
	_, err := LoadCheckpointFromFile("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestResumeFromCheckpoint_NoFile(t *testing.T) {
	config := DefaultStreamBatchConfig()
	_, err := ResumeFromCheckpoint(context.Background(), config)
	if err == nil {
		t.Error("Expected error when no checkpoint file specified")
	}
}

func TestResumeFromCheckpoint_FileNotFound(t *testing.T) {
	config := DefaultStreamBatchConfig()
	config.CheckpointFile = "/nonexistent/checkpoint.json"
	_, err := ResumeFromCheckpoint(context.Background(), config)
	if err == nil {
		t.Error("Expected error for nonexistent checkpoint file")
	}
}

func TestStreamBatchResult_Fields(t *testing.T) {
	result := &StreamBatchResult{
		Domain:      "example.com",
		RawResponse: "raw whois data",
		Latency:     150,
		RetryCount:  2,
		FromCache:   true,
	}

	if result.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", result.Domain)
	}
	if result.Latency != 150 {
		t.Errorf("Latency = %d, want 150", result.Latency)
	}
	if result.RetryCount != 2 {
		t.Errorf("RetryCount = %d, want 2", result.RetryCount)
	}
	if !result.FromCache {
		t.Error("FromCache should be true")
	}
}

func TestStreamBatchStats_EstimatedRemaining(t *testing.T) {
	config := DefaultStreamBatchConfig()
	processor := NewStreamBatchProcessor(config)
	processor.totalTasks = 100
	processor.startTime = time.Now().Add(-10 * time.Second)
	atomic.StoreInt64(&processor.completed, 50)
	processor.totalLatency = 5000

	stats := processor.GetStats()

	if stats.Completed != 50 {
		t.Errorf("Completed = %d, want 50", stats.Completed)
	}
	if stats.AvgLatency != 100 { // 5000/50
		t.Errorf("AvgLatency = %d, want 100", stats.AvgLatency)
	}
	if stats.EstimatedRemaining <= 0 {
		t.Error("EstimatedRemaining should be positive")
	}
}

func TestCollectResults(t *testing.T) {
	ch := make(chan *StreamBatchResult, 3)
	ch <- &StreamBatchResult{Domain: "a.com"}
	ch <- &StreamBatchResult{Domain: "b.com"}
	ch <- &StreamBatchResult{Domain: "c.com"}
	close(ch)

	results := CollectResults(ch)

	if len(results) != 3 {
		t.Errorf("Results count = %d, want 3", len(results))
	}
}

func TestCollectResults_Empty(t *testing.T) {
	ch := make(chan *StreamBatchResult)
	close(ch)

	results := CollectResults(ch)

	if len(results) != 0 {
		t.Errorf("Results count = %d, want 0", len(results))
	}
}

func TestCheckpointResult_Serialization(t *testing.T) {
	cr := &CheckpointResult{
		RawResponse: "test response",
		Error:       "connection timeout",
		Latency:     500,
		RetryCount:  3,
		FromCache:   false,
	}

	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("Failed to marshal CheckpointResult: %v", err)
	}

	var loaded CheckpointResult
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal CheckpointResult: %v", err)
	}

	if loaded.RawResponse != "test response" {
		t.Errorf("RawResponse = %s, want 'test response'", loaded.RawResponse)
	}
	if loaded.Error != "connection timeout" {
		t.Errorf("Error = %s, want 'connection timeout'", loaded.Error)
	}
	if loaded.Latency != 500 {
		t.Errorf("Latency = %d, want 500", loaded.Latency)
	}
	if loaded.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", loaded.RetryCount)
	}
}

func TestCheckpoint_FullSerialization(t *testing.T) {
	checkpoint := &Checkpoint{
		BatchID:   "batch-123",
		CreatedAt: "2024-06-15T10:30:00Z",
		AllDomains: []string{"a.com", "b.com", "c.com", "d.com", "e.com"},
		CompletedDomains: map[string]bool{
			"a.com": true,
			"b.com": true,
		},
		Results: map[string]*CheckpointResult{
			"a.com": {RawResponse: "data-a", Latency: 100},
			"b.com": {Error: "timeout", Latency: 5000, RetryCount: 3},
		},
		TotalTasks:   5,
		SuccessCount: 1,
		FailureCount: 1,
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal checkpoint: %v", err)
	}

	var loaded Checkpoint
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal checkpoint: %v", err)
	}

	if loaded.BatchID != "batch-123" {
		t.Errorf("BatchID = %s, want batch-123", loaded.BatchID)
	}
	if len(loaded.AllDomains) != 5 {
		t.Errorf("AllDomains count = %d, want 5", len(loaded.AllDomains))
	}
	if len(loaded.CompletedDomains) != 2 {
		t.Errorf("CompletedDomains count = %d, want 2", len(loaded.CompletedDomains))
	}
	if loaded.TotalTasks != 5 {
		t.Errorf("TotalTasks = %d, want 5", loaded.TotalTasks)
	}
}

func TestStreamBatchProcessor_CheckpointAutoSave(t *testing.T) {
	tmpDir := t.TempDir()
	checkpointFile := filepath.Join(tmpDir, "autosave.json")

	config := DefaultStreamBatchConfig()
	config.CheckpointFile = checkpointFile
	config.CheckpointInterval = 2 // Save every 2 completions
	config.Concurrency = 1
	config.Timeout = 1 // Short timeout to fail fast

	processor := NewStreamBatchProcessor(config)
	processor.checkpoint = processor.newCheckpoint([]string{"a.com", "b.com"})

	// Simulate completions
	processor.mu.Lock()
	processor.checkpoint.CompletedDomains["a.com"] = true
	processor.checkpoint.Results["a.com"] = &CheckpointResult{Latency: 100}
	processor.checkpoint.SuccessCount = 1
	processor.mu.Unlock()
	atomic.AddInt64(&processor.completed, 1)

	// Trigger auto-save (completed % interval == 0)
	processor.saveCheckpoint()

	if _, err := os.Stat(checkpointFile); os.IsNotExist(err) {
		t.Error("Checkpoint file should exist after auto-save")
	}
}

func TestDefaultStreamBatchConfig(t *testing.T) {
	config := DefaultStreamBatchConfig()

	if config.Concurrency != 5 {
		t.Errorf("Default Concurrency = %d, want 5", config.Concurrency)
	}
	if config.Timeout != 10 {
		t.Errorf("Default Timeout = %d, want 10", config.Timeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Default MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.CheckpointInterval != 10 {
		t.Errorf("Default CheckpointInterval = %d, want 10", config.CheckpointInterval)
	}
	if config.QueryDelay != 200 {
		t.Errorf("Default QueryDelay = %d, want 200", config.QueryDelay)
	}
}

func BenchmarkCheckpointSerialization(b *testing.B) {
	checkpoint := &Checkpoint{
		BatchID:          "bench-batch",
		CreatedAt:        "2024-01-01T00:00:00Z",
		AllDomains:       make([]string, 1000),
		CompletedDomains: make(map[string]bool),
		Results:          make(map[string]*CheckpointResult),
	}

	for i := 0; i < 1000; i++ {
		domain := "site" + string(rune('0'+i%10)) + string(rune('0'+i/10%10)) + ".com"
		checkpoint.AllDomains[i] = domain
		if i < 500 {
			checkpoint.CompletedDomains[domain] = true
			checkpoint.Results[domain] = &CheckpointResult{Latency: int64(i * 10)}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(checkpoint)
	}
}

func BenchmarkNewStreamBatchProcessor(b *testing.B) {
	config := DefaultStreamBatchConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewStreamBatchProcessor(config)
	}
}
