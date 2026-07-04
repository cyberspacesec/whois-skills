package whois

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// WhoisLibraryConfig WHOIS库统一配置
// 将所有子模块的配置整合到一个结构体中
type WhoisLibraryConfig struct {
	mu sync.RWMutex

	// 查询配置
	Query WhoisQueryConfig `json:"query"`

	// 缓存配置
	Cache WhoisCacheConfig `json:"cache"`

	// 代理配置
	Proxy WhoisProxyConfig `json:"proxy"`

	// 限流配置
	RateLimit WhoisRateLimitConfig `json:"rate_limit"`

	// 批量查询配置
	Batch WhoisBatchConfig `json:"batch"`

	// 监控配置
	Monitor WhoisMonitorConfig `json:"monitor"`

	// 调度器配置
	Scheduler WhoisSchedulerConfig `json:"scheduler"`

	// 可观测性配置
	Observability WhoisObservabilityConfig `json:"observability"`

	// 持久化存储配置（让上层把 WHOIS 数据落本地/Redis/ES 等）
	Storage StorageConfig `json:"storage"`

	// ASN BGP 关系查询配置（上游/下游/对等 AS）
	ASNRelation ASNRelationConfig `json:"asn_relation"`

	// 历史 WHOIS 快照配置
	History HistoryConfig `json:"history"`

	// 告警持久化配置
	AlertStorage AlertStorageConfig `json:"alert_storage"`

	// 监控状态持久化配置
	MonitorState MonitorStateConfig `json:"monitor_state"`

	// 日志配置
	Log WhoisLogConfig `json:"log"`
}

// WhoisQueryConfig 查询配置
type WhoisQueryConfig struct {
	// 默认超时（秒）
	Timeout int `json:"timeout"`

	// 最大重试次数
	MaxRetries int `json:"max_retries"`

	// 重试间隔（毫秒）
	RetryInterval int `json:"retry_interval_ms"`

	// 是否使用代理
	UseProxy bool `json:"use_proxy"`

	// 是否跟随WHOIS引导
	FollowReferral bool `json:"follow_referral"`

	// 最大引导次数
	MaxReferrals int `json:"max_referrals"`

	// 是否验证结果
	ValidateResult bool `json:"validate_result"`

	// 域间查询间隔（毫秒）
	QueryDelay int `json:"query_delay_ms"`
}

// WhoisCacheConfig 缓存配置
type WhoisCacheConfig struct {
	// 是否启用缓存
	Enabled bool `json:"enabled"`

	// 缓存类型 (local/redis)
	Type string `json:"type"`

	// 本地缓存最大条目数
	MaxEntries int `json:"max_entries"`

	// 默认TTL（分钟）
	DefaultTTLMinutes int `json:"default_ttl_minutes"`

	// Redis地址
	RedisAddr string `json:"redis_addr,omitempty"`

	// Redis密码
	RedisPassword string `json:"redis_password,omitempty"`

	// Redis数据库
	RedisDB int `json:"redis_db,omitempty"`
}

// WhoisProxyConfig 代理配置
type WhoisProxyConfig struct {
	// 是否启用代理池
	Enabled bool `json:"enabled"`

	// SOCKS5代理地址
	SOCKS5Addr string `json:"socks5_addr,omitempty"`

	// HTTP代理地址
	HTTPAddr string `json:"http_addr,omitempty"`

	// 代理用户名
	Username string `json:"username,omitempty"`

	// 代理密码
	Password string `json:"password,omitempty"`

	// 代理列表文件路径
	ProxyFile string `json:"proxy_file,omitempty"`
}

// WhoisRateLimitConfig 限流配置
type WhoisRateLimitConfig struct {
	// 是否启用限流
	Enabled bool `json:"enabled"`

	// 全局速率（每秒请求数）
	GlobalRate float64 `json:"global_rate"`

	// 每服务器速率
	PerServerRate map[string]float64 `json:"per_server_rate,omitempty"`

	// 突发大小
	BurstSize int `json:"burst_size"`
}

// WhoisBatchConfig 批量查询配置
type WhoisBatchConfig struct {
	// 并发数
	Concurrency int `json:"concurrency"`

	// 断点续查文件路径
	CheckpointFile string `json:"checkpoint_file,omitempty"`

	// 断点保存间隔
	CheckpointInterval int `json:"checkpoint_interval"`

	// 查询超时（秒）
	Timeout int `json:"timeout"`

	// 最大重试次数
	MaxRetries int `json:"max_retries"`
}

// WhoisMonitorConfig 监控配置
type WhoisMonitorConfig struct {
	// 是否启用监控
	Enabled bool `json:"enabled"`

	// 检查间隔（分钟）
	CheckIntervalMinutes int `json:"check_interval_minutes"`

	// 到期预警天数
	ExpiryWarningDays int `json:"expiry_warning_days"`

	// 到期紧急天数
	ExpiryCriticalDays int `json:"expiry_critical_days"`

	// 监控状态变更
	WatchStatusChange bool `json:"watch_status_change"`

	// 监控注册人变更
	WatchRegistrantChange bool `json:"watch_registrant_change"`

	// 监控NS变更
	WatchNSChange bool `json:"watch_ns_change"`
}

// WhoisSchedulerConfig 调度器配置
type WhoisSchedulerConfig struct {
	// 默认查询间隔（毫秒）
	DefaultIntervalMs int `json:"default_interval_ms"`

	// 最小查询间隔
	MinIntervalMs int `json:"min_interval_ms"`

	// 最大查询间隔
	MaxIntervalMs int `json:"max_interval_ms"`

	// 最大并发数
	MaxConcurrency int `json:"max_concurrency"`

	// 自适应调整因子
	AdaptFactor float64 `json:"adapt_factor"`

	// 退避初始时间（毫秒）
	BackoffInitialMs int `json:"backoff_initial_ms"`

	// 退避最大时间（毫秒）
	BackoffMaxMs int `json:"backoff_max_ms"`

	// 退避倍数
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// 不健康阈值
	UnhealthyThreshold int `json:"unhealthy_threshold"`
}

// WhoisObservabilityConfig 可观测性配置
type WhoisObservabilityConfig struct {
	// 是否启用
	Enabled bool `json:"enabled"`

	// 提供者列表 (prometheus/opentelemetry/nop)
	Providers []string `json:"providers"`

	// Prometheus指标路径
	PrometheusPath string `json:"prometheus_path,omitempty"`

	// Prometheus端口
	PrometheusPort int `json:"prometheus_port,omitempty"`

	// OTLP导出端点
	OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
}

// WhoisLogConfig 日志配置
type WhoisLogConfig struct {
	// 日志级别 (debug/info/warn/error)
	Level string `json:"level"`

	// 日志格式 (text/json)
	Format string `json:"format"`

	// 日志输出文件
	OutputFile string `json:"output_file,omitempty"`
}

// DefaultWhoisLibraryConfig 默认库配置
func DefaultWhoisLibraryConfig() WhoisLibraryConfig {
	return WhoisLibraryConfig{
		Query: WhoisQueryConfig{
			Timeout:         10,
			MaxRetries:      3,
			RetryInterval:   1000,
			UseProxy:        false,
			FollowReferral:  true,
			MaxReferrals:    3,
			ValidateResult:  false,
			QueryDelay:      200,
		},
		Cache: WhoisCacheConfig{
			Enabled:            true,
			Type:              "local",
			MaxEntries:         10000,
			DefaultTTLMinutes: 60,
			RedisAddr:         "localhost:6379",
			RedisDB:           0,
		},
		Proxy: WhoisProxyConfig{
			Enabled: false,
		},
		RateLimit: WhoisRateLimitConfig{
			Enabled:    true,
			GlobalRate: 10.0,
			BurstSize:  20,
		},
		Batch: WhoisBatchConfig{
			Concurrency:        5,
			CheckpointInterval: 10,
			Timeout:            10,
			MaxRetries:         3,
		},
		Monitor: WhoisMonitorConfig{
			Enabled:              false,
			CheckIntervalMinutes: 60,
			ExpiryWarningDays:    30,
			ExpiryCriticalDays:   7,
			WatchStatusChange:    true,
			WatchRegistrantChange: true,
			WatchNSChange:       true,
		},
		Scheduler: WhoisSchedulerConfig{
			DefaultIntervalMs:  200,
			MinIntervalMs:     50,
			MaxIntervalMs:     5000,
			MaxConcurrency:    5,
			AdaptFactor:       0.3,
			BackoffInitialMs:  1000,
			BackoffMaxMs:       60000,
			BackoffMultiplier:  2.0,
			UnhealthyThreshold: 3,
		},
		Observability: WhoisObservabilityConfig{
			Enabled:        false,
			Providers:      []string{"nop"},
			PrometheusPath: "/metrics",
			PrometheusPort: 9090,
		},
		Storage: StorageConfig{
			Enabled:   false,
			Type:      "local",
			Directory: "data/storage",
			RedisConfig: &RedisConfig{
				Addr:     "localhost:6379",
				DB:       0,
				PoolSize: 10,
			},
		},
		ASNRelation: ASNRelationConfig{
			Enabled:  false,
			Type:     "local",
			FilePath: "data/as-rel.txt",
		},
		History: HistoryConfig{
			Enabled:          false,
			Type:             "local",
			Directory:        "data/history",
			MaxRetentionDays: 365,
		},
		AlertStorage: AlertStorageConfig{
			Enabled:   false,
			Type:      "local",
			Directory: "data/alerts",
		},
		MonitorState: MonitorStateConfig{
			Enabled:   false,
			Type:      "local",
			Directory: "data/monitor",
		},
		Log: WhoisLogConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// globalLibConfig 全局库配置
var globalLibConfig *WhoisLibraryConfig
var libConfigOnce sync.Once

// GetWhoisLibraryConfig 获取全局库配置
func GetWhoisLibraryConfig() *WhoisLibraryConfig {
	libConfigOnce.Do(func() {
		globalLibConfig = libConfigFromEnv()
		if globalLibConfig == nil {
			cfg := DefaultWhoisLibraryConfig()
			globalLibConfig = &cfg
		}
	})
	return globalLibConfig
}

// SetWhoisLibraryConfig 设置全局库配置
func SetWhoisLibraryConfig(cfg *WhoisLibraryConfig) {
	if cfg == nil {
		return
	}
	libConfigOnce.Do(func() {})
	globalLibConfig = cfg
}

// libConfigFromEnv 从环境变量读取配置
func libConfigFromEnv() *WhoisLibraryConfig {
	configFile := os.Getenv("WHOIS_CONFIG_FILE")
	if configFile == "" {
		return nil
	}
	return LoadWhoisLibraryConfigFromFile(configFile)
}

// LoadWhoisLibraryConfigFromFile 从文件加载配置（支持JSON和YAML格式）
func LoadWhoisLibraryConfigFromFile(path string) *WhoisLibraryConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		logrus.Warnf("读取配置文件失败 %s: %v", path, err)
		return nil
	}

	cfg := DefaultWhoisLibraryConfig()

	// 根据文件扩展名选择解析器
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			logrus.Warnf("解析YAML配置文件失败 %s: %v", path, err)
			return nil
		}
	default:
		if err := json.Unmarshal(data, &cfg); err != nil {
			logrus.Warnf("解析JSON配置文件失败 %s: %v", path, err)
			return nil
		}
	}

	return &cfg
}

// LoadYAMLConfig 从YAML文件加载应用配置
// 返回一个可被main.go直接使用的配置映射
func LoadYAMLConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := DefaultAppConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析YAML配置文件失败: %w", err)
	}

	return cfg, nil
}

// AppConfig 应用顶层配置（对应config.yaml）
type AppConfig struct {
	// HTTP服务配置
	Server AppConfigServer `yaml:"server" json:"server"`

	// 日志配置
	Log AppConfigLog `yaml:"log" json:"log"`

	// 缓存配置
	Cache AppConfigCache `yaml:"cache" json:"cache"`

	// 代理配置
	Proxy AppConfigProxy `yaml:"proxy" json:"proxy"`

	// 监控配置
	Metrics AppConfigMetrics `yaml:"metrics" json:"metrics"`

	// 告警配置
	Alerts AppConfigAlerts `yaml:"alerts" json:"alerts"`
}

// AppConfigServer HTTP服务配置
type AppConfigServer struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// AppConfigLog 日志配置
type AppConfigLog struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
}

// AppConfigCache 缓存配置
type AppConfigCache struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Type       string `yaml:"type" json:"type"`
	TTL        int64  `yaml:"ttl" json:"ttl"`
	Warmup     bool   `yaml:"warmup" json:"warmup"`
	WarmupFile string `yaml:"warmup_file" json:"warmup_file"`
}

// AppConfigProxy 代理配置
type AppConfigProxy struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	File    string `yaml:"file" json:"file"`
}

// AppConfigMetrics 监控配置
type AppConfigMetrics struct {
	Enabled  bool  `yaml:"enabled" json:"enabled"`
	Interval int64 `yaml:"interval" json:"interval"`
}

// AppConfigAlerts 告警配置
type AppConfigAlerts struct {
	Enabled  bool  `yaml:"enabled" json:"enabled"`
	Interval int64 `yaml:"interval" json:"interval"`
}

// DefaultAppConfig 默认应用配置
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Server: AppConfigServer{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Log: AppConfigLog{
			Level:  "info",
			Format: "text",
		},
		Cache: AppConfigCache{
			Enabled: true,
			Type:    "local",
			TTL:     3600,
		},
		Proxy: AppConfigProxy{
			Enabled: false,
			File:    "config/proxies.json",
		},
		Metrics: AppConfigMetrics{
			Enabled:  true,
			Interval: 60,
		},
		Alerts: AppConfigAlerts{
			Enabled:  true,
			Interval: 60,
		},
	}
}

// SaveWhoisLibraryConfigToFile 保存配置到文件
func SaveWhoisLibraryConfigToFile(cfg *WhoisLibraryConfig, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// ValidateWhoisLibraryConfig 验证配置有效性
func ValidateWhoisLibraryConfig(cfg *WhoisLibraryConfig) error {
	if cfg == nil {
		return fmt.Errorf("配置不能为空")
	}

	if cfg.Query.Timeout <= 0 {
		return fmt.Errorf("查询超时必须大于0，当前: %d", cfg.Query.Timeout)
	}
	if cfg.Query.MaxRetries < 0 {
		return fmt.Errorf("最大重试次数不能为负，当前: %d", cfg.Query.MaxRetries)
	}
	if cfg.Cache.MaxEntries <= 0 && cfg.Cache.Enabled {
		return fmt.Errorf("缓存最大条目数必须大于0，当前: %d", cfg.Cache.MaxEntries)
	}
	if cfg.Cache.DefaultTTLMinutes <= 0 && cfg.Cache.Enabled {
		return fmt.Errorf("缓存TTL必须大于0，当前: %d", cfg.Cache.DefaultTTLMinutes)
	}
	if cfg.RateLimit.GlobalRate <= 0 && cfg.RateLimit.Enabled {
		return fmt.Errorf("全局速率必须大于0，当前: %f", cfg.RateLimit.GlobalRate)
	}
	if cfg.Batch.Concurrency <= 0 {
		return fmt.Errorf("批量查询并发数必须大于0，当前: %d", cfg.Batch.Concurrency)
	}
	if cfg.Monitor.CheckIntervalMinutes <= 0 && cfg.Monitor.Enabled {
		return fmt.Errorf("监控检查间隔必须大于0，当前: %d", cfg.Monitor.CheckIntervalMinutes)
	}
	if cfg.Scheduler.DefaultIntervalMs <= 0 {
		return fmt.Errorf("调度器默认间隔必须大于0，当前: %d", cfg.Scheduler.DefaultIntervalMs)
	}

	// 校验持久化存储配置
	if cfg.Storage.Enabled {
		if cfg.Storage.Type != "local" && cfg.Storage.Type != "redis" {
			return fmt.Errorf("存储类型必须是 local 或 redis，当前: %s", cfg.Storage.Type)
		}
		if cfg.Storage.Type == "local" && cfg.Storage.Directory == "" {
			// 允许空，会使用默认目录
		}
		if cfg.Storage.Type == "redis" && cfg.Storage.RedisConfig == nil {
			return fmt.Errorf("Redis 存储必须提供 RedisConfig")
		}
	}

	// 校验 AS 关系配置
	if cfg.ASNRelation.Enabled {
		if cfg.ASNRelation.Type != "local" && cfg.ASNRelation.Type != "api" {
			return fmt.Errorf("AS 关系数据源类型必须是 local 或 api，当前: %s", cfg.ASNRelation.Type)
		}
	}

	// 校验历史快照配置
	if cfg.History.Enabled {
		if cfg.History.Type != "local" && cfg.History.Type != "custom" {
			return fmt.Errorf("历史存储类型必须是 local 或 custom，当前: %s", cfg.History.Type)
		}
	}

	// 校验告警存储配置
	if cfg.AlertStorage.Enabled {
		if cfg.AlertStorage.Type != "local" && cfg.AlertStorage.Type != "redis" {
			return fmt.Errorf("告警存储类型必须是 local 或 redis，当前: %s", cfg.AlertStorage.Type)
		}
	}

	// 校验监控状态配置
	if cfg.MonitorState.Enabled {
		if cfg.MonitorState.Type != "local" && cfg.MonitorState.Type != "redis" {
			return fmt.Errorf("监控状态存储类型必须是 local 或 redis，当前: %s", cfg.MonitorState.Type)
		}
	}

	return nil
}

// ApplyWhoisLibraryConfig 应用配置
func ApplyWhoisLibraryConfig(cfg *WhoisLibraryConfig) error {
	if err := ValidateWhoisLibraryConfig(cfg); err != nil {
		return err
	}

	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		logrus.Warnf("无效日志级别 %s，使用默认info", cfg.Log.Level)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	if cfg.Log.Format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}

	// 初始化持久化存储（如启用）
	if cfg.Storage.Enabled {
		if err := InitStorageFromConfig(&cfg.Storage); err != nil {
			logrus.Warnf("初始化存储失败: %v", err)
		} else {
			logrus.Infof("存储已启用: 类型=%s", cfg.Storage.Type)
		}
	}

	// 初始化 ASN BGP 关系数据源（如启用）
	if cfg.ASNRelation.Enabled {
		if err := InitASNRelationFromConfig(&cfg.ASNRelation); err != nil {
			logrus.Warnf("初始化 AS 关系数据源失败: %v", err)
		} else {
			logrus.Infof("AS 关系查询已启用: 类型=%s", cfg.ASNRelation.Type)
		}
	}

	// 初始化历史 WHOIS 快照存储（如启用）
	if cfg.History.Enabled {
		if err := InitHistoryFromConfig(&cfg.History); err != nil {
			logrus.Warnf("初始化历史快照存储失败: %v", err)
		} else {
			logrus.Infof("历史快照已启用: 类型=%s", cfg.History.Type)
		}
	}

	// 初始化告警持久化（如启用）
	if cfg.AlertStorage.Enabled {
		if err := InitAlertStorageFromConfig(&cfg.AlertStorage); err != nil {
			logrus.Warnf("初始化告警存储失败: %v", err)
		} else {
			logrus.Infof("告警持久化已启用: 类型=%s", cfg.AlertStorage.Type)
		}
	}

	// 初始化监控状态持久化（如启用）
	if cfg.MonitorState.Enabled {
		if err := InitMonitorStateFromConfig(&cfg.MonitorState); err != nil {
			logrus.Warnf("初始化监控状态存储失败: %v", err)
		} else {
			logrus.Infof("监控状态持久化已启用: 类型=%s", cfg.MonitorState.Type)
		}
	}

	logrus.Infof("配置已应用: 查询超时=%ds, 缓存=%v, 限流=%v",
		cfg.Query.Timeout, cfg.Cache.Enabled, cfg.RateLimit.Enabled)

	return nil
}

// WhoisLibraryConfigSummary 配置摘要
func WhoisLibraryConfigSummary(cfg *WhoisLibraryConfig) string {
	if cfg == nil {
		return "配置为空"
	}

	return fmt.Sprintf(
		"查询: timeout=%ds retries=%d proxy=%v | "+
			"缓存: enabled=%v type=%s ttl=%dm | "+
			"限流: enabled=%v rate=%.1f/s | "+
			"批量: concurrency=%d | "+
			"监控: enabled=%v | "+
			"调度: interval=%dms | "+
			"存储: enabled=%v type=%s | "+
			"ASN关系: enabled=%v type=%s | "+
			"历史: enabled=%v type=%s | "+
			"告警存储: enabled=%v | "+
			"监控状态: enabled=%v",
		cfg.Query.Timeout, cfg.Query.MaxRetries, cfg.Query.UseProxy,
		cfg.Cache.Enabled, cfg.Cache.Type, cfg.Cache.DefaultTTLMinutes,
		cfg.RateLimit.Enabled, cfg.RateLimit.GlobalRate,
		cfg.Batch.Concurrency,
		cfg.Monitor.Enabled,
		cfg.Scheduler.DefaultIntervalMs,
		cfg.Storage.Enabled, cfg.Storage.Type,
		cfg.ASNRelation.Enabled, cfg.ASNRelation.Type,
		cfg.History.Enabled, cfg.History.Type,
		cfg.AlertStorage.Enabled,
		cfg.MonitorState.Enabled,
	)
}

// MergeWhoisLibraryConfigs 合并配置
func MergeWhoisLibraryConfigs(base *WhoisLibraryConfig, overrides ...*WhoisLibraryConfig) *WhoisLibraryConfig {
	result := &WhoisLibraryConfig{
		Query:         base.Query,
		Cache:         base.Cache,
		Proxy:         base.Proxy,
		RateLimit:     base.RateLimit,
		Batch:         base.Batch,
		Monitor:       base.Monitor,
		Scheduler:     base.Scheduler,
		Observability: base.Observability,
		Log:           base.Log,
	}

	for _, override := range overrides {
		if override == nil {
			continue
		}

		if override.Query.Timeout > 0 {
			result.Query.Timeout = override.Query.Timeout
		}
		if override.Query.MaxRetries > 0 {
			result.Query.MaxRetries = override.Query.MaxRetries
		}
		if override.Query.RetryInterval > 0 {
			result.Query.RetryInterval = override.Query.RetryInterval
		}
		if override.Query.QueryDelay > 0 {
			result.Query.QueryDelay = override.Query.QueryDelay
		}
		if override.Cache.Type != "" {
			result.Cache.Type = override.Cache.Type
		}
		if override.Cache.MaxEntries > 0 {
			result.Cache.MaxEntries = override.Cache.MaxEntries
		}
		if override.Cache.DefaultTTLMinutes > 0 {
			result.Cache.DefaultTTLMinutes = override.Cache.DefaultTTLMinutes
		}
		if override.RateLimit.GlobalRate > 0 {
			result.RateLimit.GlobalRate = override.RateLimit.GlobalRate
		}
		if len(override.RateLimit.PerServerRate) > 0 {
			result.RateLimit.PerServerRate = override.RateLimit.PerServerRate
		}
		if override.Batch.Concurrency > 0 {
			result.Batch.Concurrency = override.Batch.Concurrency
		}
		if override.Batch.CheckpointFile != "" {
			result.Batch.CheckpointFile = override.Batch.CheckpointFile
		}
		if override.Monitor.CheckIntervalMinutes > 0 {
			result.Monitor.CheckIntervalMinutes = override.Monitor.CheckIntervalMinutes
		}
		if override.Scheduler.DefaultIntervalMs > 0 {
			result.Scheduler.DefaultIntervalMs = override.Scheduler.DefaultIntervalMs
		}
		if override.Scheduler.MaxConcurrency > 0 {
			result.Scheduler.MaxConcurrency = override.Scheduler.MaxConcurrency
		}
		if override.Log.Level != "" {
			result.Log.Level = override.Log.Level
		}
		if override.Log.Format != "" {
			result.Log.Format = override.Log.Format
		}
	}

	return result
}
