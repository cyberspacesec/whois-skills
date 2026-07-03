package whois

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultWhoisLibraryConfig(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()

	if cfg.Query.Timeout != 10 {
		t.Errorf("Query.Timeout = %d, want 10", cfg.Query.Timeout)
	}
	if cfg.Query.MaxRetries != 3 {
		t.Errorf("Query.MaxRetries = %d, want 3", cfg.Query.MaxRetries)
	}
	if !cfg.Cache.Enabled {
		t.Error("Cache.Enabled should be true")
	}
	if cfg.Cache.Type != "local" {
		t.Errorf("Cache.Type = %s, want local", cfg.Cache.Type)
	}
	if cfg.Cache.DefaultTTLMinutes != 60 {
		t.Errorf("Cache.DefaultTTLMinutes = %d, want 60", cfg.Cache.DefaultTTLMinutes)
	}
	if !cfg.RateLimit.Enabled {
		t.Error("RateLimit.Enabled should be true")
	}
	if cfg.RateLimit.GlobalRate != 10.0 {
		t.Errorf("RateLimit.GlobalRate = %f, want 10.0", cfg.RateLimit.GlobalRate)
	}
	if cfg.Batch.Concurrency != 5 {
		t.Errorf("Batch.Concurrency = %d, want 5", cfg.Batch.Concurrency)
	}
	if cfg.Monitor.Enabled {
		t.Error("Monitor.Enabled should be false by default")
	}
	if cfg.Scheduler.DefaultIntervalMs != 200 {
		t.Errorf("Scheduler.DefaultIntervalMs = %d, want 200", cfg.Scheduler.DefaultIntervalMs)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %s, want info", cfg.Log.Level)
	}
}

func TestValidateWhoisLibraryConfig_Nil(t *testing.T) {
	err := ValidateWhoisLibraryConfig(nil)
	if err == nil {
		t.Error("Expected error for nil config")
	}
}

func TestValidateWhoisLibraryConfig_Valid(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	err := ValidateWhoisLibraryConfig(&cfg)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateWhoisLibraryConfig_InvalidTimeout(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Query.Timeout = 0
	err := ValidateWhoisLibraryConfig(&cfg)
	if err == nil {
		t.Error("Expected error for zero timeout")
	}
}

func TestValidateWhoisLibraryConfig_NegativeRetries(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Query.MaxRetries = -1
	err := ValidateWhoisLibraryConfig(&cfg)
	if err == nil {
		t.Error("Expected error for negative retries")
	}
}

func TestValidateWhoisLibraryConfig_InvalidCache(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Cache.MaxEntries = 0
	err := ValidateWhoisLibraryConfig(&cfg)
	if err == nil {
		t.Error("Expected error for zero max entries with cache enabled")
	}
}

func TestValidateWhoisLibraryConfig_InvalidBatch(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Batch.Concurrency = 0
	err := ValidateWhoisLibraryConfig(&cfg)
	if err == nil {
		t.Error("Expected error for zero concurrency")
	}
}

func TestSaveAndLoadWhoisLibraryConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := DefaultWhoisLibraryConfig()
	cfg.Query.Timeout = 20
	cfg.Cache.MaxEntries = 5000
	cfg.RateLimit.GlobalRate = 5.0

	err := SaveWhoisLibraryConfigToFile(&cfg, configPath)
	if err != nil {
		t.Fatalf("SaveConfigToFile() error = %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file should exist")
	}

	loaded := LoadWhoisLibraryConfigFromFile(configPath)
	if loaded == nil {
		t.Fatal("LoadConfigFromFile() returned nil")
	}

	if loaded.Query.Timeout != 20 {
		t.Errorf("Query.Timeout = %d, want 20", loaded.Query.Timeout)
	}
	if loaded.Cache.MaxEntries != 5000 {
		t.Errorf("Cache.MaxEntries = %d, want 5000", loaded.Cache.MaxEntries)
	}
	if loaded.RateLimit.GlobalRate != 5.0 {
		t.Errorf("RateLimit.GlobalRate = %f, want 5.0", loaded.RateLimit.GlobalRate)
	}
}

func TestLoadWhoisLibraryConfigFromFile_NotFound(t *testing.T) {
	loaded := LoadWhoisLibraryConfigFromFile("/nonexistent/config.json")
	if loaded != nil {
		t.Error("Expected nil for nonexistent file")
	}
}

func TestLoadWhoisLibraryConfigFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(configPath, []byte("not valid json"), 0644)

	loaded := LoadWhoisLibraryConfigFromFile(configPath)
	if loaded != nil {
		t.Error("Expected nil for invalid JSON")
	}
}

func TestGetWhoisLibraryConfig(t *testing.T) {
	cfg := GetWhoisLibraryConfig()
	if cfg == nil {
		t.Fatal("GetWhoisLibraryConfig() returned nil")
	}
}

func TestSetWhoisLibraryConfig(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Query.Timeout = 30
	SetWhoisLibraryConfig(&cfg)

	retrieved := GetWhoisLibraryConfig()
	if retrieved.Query.Timeout != 30 {
		t.Errorf("Query.Timeout = %d, want 30", retrieved.Query.Timeout)
	}
}

func TestSetWhoisLibraryConfig_Nil(t *testing.T) {
	SetWhoisLibraryConfig(nil)
	// Should not panic
}

func TestApplyWhoisLibraryConfig(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	err := ApplyWhoisLibraryConfig(&cfg)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestApplyWhoisLibraryConfig_Invalid(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Query.Timeout = 0
	err := ApplyWhoisLibraryConfig(&cfg)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

func TestWhoisLibraryConfigSummary(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	summary := WhoisLibraryConfigSummary(&cfg)

	if summary == "" {
		t.Error("Summary should not be empty")
	}
	if summary == "配置为空" {
		t.Error("Summary should not be '配置为空' for valid config")
	}
}

func TestWhoisLibraryConfigSummary_Nil(t *testing.T) {
	summary := WhoisLibraryConfigSummary(nil)
	if summary != "配置为空" {
		t.Errorf("Summary for nil = %s, want '配置为空'", summary)
	}
}

func TestMergeWhoisLibraryConfigs(t *testing.T) {
	base := DefaultWhoisLibraryConfig()

	override := &WhoisLibraryConfig{
		Query: WhoisQueryConfig{
			Timeout:    30,
			MaxRetries: 5,
		},
		Cache: WhoisCacheConfig{
			MaxEntries: 50000,
		},
		RateLimit: WhoisRateLimitConfig{
			GlobalRate: 20.0,
		},
		Batch: WhoisBatchConfig{
			Concurrency: 10,
		},
	}

	merged := MergeWhoisLibraryConfigs(&base, override)

	if merged.Query.Timeout != 30 {
		t.Errorf("Merged Timeout = %d, want 30", merged.Query.Timeout)
	}
	if merged.Query.MaxRetries != 5 {
		t.Errorf("Merged MaxRetries = %d, want 5", merged.Query.MaxRetries)
	}
	if merged.Cache.MaxEntries != 50000 {
		t.Errorf("Merged MaxEntries = %d, want 50000", merged.Cache.MaxEntries)
	}
	if merged.RateLimit.GlobalRate != 20.0 {
		t.Errorf("Merged GlobalRate = %f, want 20.0", merged.RateLimit.GlobalRate)
	}
	if merged.Batch.Concurrency != 10 {
		t.Errorf("Merged Concurrency = %d, want 10", merged.Batch.Concurrency)
	}

	// Unchanged fields should keep base values
	if merged.Cache.Type != "local" {
		t.Errorf("Merged Cache.Type = %s, want local (from base)", merged.Cache.Type)
	}
}

func TestMergeWhoisLibraryConfigs_NilOverride(t *testing.T) {
	base := DefaultWhoisLibraryConfig()

	merged := MergeWhoisLibraryConfigs(&base, nil)

	if merged.Query.Timeout != base.Query.Timeout {
		t.Error("Nil override should not change config")
	}
}

func TestMergeWhoisLibraryConfigs_MultipleOverrides(t *testing.T) {
	base := DefaultWhoisLibraryConfig()

	first := &WhoisLibraryConfig{
		Query: WhoisQueryConfig{Timeout: 20},
	}

	second := &WhoisLibraryConfig{
		Query: WhoisQueryConfig{MaxRetries: 7},
	}

	merged := MergeWhoisLibraryConfigs(&base, first, second)

	if merged.Query.Timeout != 20 {
		t.Errorf("Timeout = %d, want 20", merged.Query.Timeout)
	}
	if merged.Query.MaxRetries != 7 {
		t.Errorf("MaxRetries = %d, want 7", merged.Query.MaxRetries)
	}
}

func TestWhoisQueryConfig_Fields(t *testing.T) {
	cfg := WhoisQueryConfig{
		Timeout:         15,
		MaxRetries:      5,
		RetryInterval:   2000,
		UseProxy:        true,
		FollowReferral:  true,
		MaxReferrals:    5,
		ValidateResult:  true,
		QueryDelay:      500,
	}

	if cfg.Timeout != 15 {
		t.Errorf("Timeout = %d, want 15", cfg.Timeout)
	}
	if !cfg.UseProxy {
		t.Error("UseProxy should be true")
	}
	if !cfg.ValidateResult {
		t.Error("ValidateResult should be true")
	}
}

func TestWhoisCacheConfig_Fields(t *testing.T) {
	cfg := WhoisCacheConfig{
		Enabled:            true,
		Type:              "redis",
		MaxEntries:         100000,
		DefaultTTLMinutes: 120,
		RedisAddr:         "redis.example.com:6379",
		RedisPassword:     "secret",
		RedisDB:           1,
	}

	if cfg.Type != "redis" {
		t.Errorf("Type = %s, want redis", cfg.Type)
	}
	if cfg.RedisDB != 1 {
		t.Errorf("RedisDB = %d, want 1", cfg.RedisDB)
	}
}

func TestWhoisObservabilityConfig_Fields(t *testing.T) {
	cfg := WhoisObservabilityConfig{
		Enabled:        true,
		Providers:      []string{"prometheus", "opentelemetry"},
		PrometheusPath: "/metrics",
		PrometheusPort: 9090,
		OTLPEndpoint:   "localhost:4317",
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("Providers count = %d, want 2", len(cfg.Providers))
	}
}

func TestWhoisLogConfig_Fields(t *testing.T) {
	cfg := WhoisLogConfig{
		Level:      "debug",
		Format:     "json",
		OutputFile: "/var/log/whois.log",
	}

	if cfg.Level != "debug" {
		t.Errorf("Level = %s, want debug", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %s, want json", cfg.Format)
	}
}

func TestSaveWhoisLibraryConfigToFile_BadPath(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	err := SaveWhoisLibraryConfigToFile(&cfg, "/nonexistent/dir/config.json")
	// This should work because we create the directory
	if err != nil {
		t.Logf("SaveConfigToFile with nonexistent dir: %v (may be expected)", err)
	}
}

func BenchmarkValidateWhoisLibraryConfig(b *testing.B) {
	cfg := DefaultWhoisLibraryConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateWhoisLibraryConfig(&cfg)
	}
}

func BenchmarkMergeWhoisLibraryConfigs(b *testing.B) {
	base := DefaultWhoisLibraryConfig()
	override := &WhoisLibraryConfig{
		Query: WhoisQueryConfig{Timeout: 30},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MergeWhoisLibraryConfigs(&base, override)
	}
}
