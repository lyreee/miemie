package config

import "time"

// Config 主配置结构体
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Cache        CacheConfig        `yaml:"cache"`
	Delivery     DeliveryConfig     `yaml:"delivery"`
	Database     DatabaseConfig     `yaml:"database"`
	User         UserConfig         `yaml:"user"`
	Performance  PerformanceConfig  `yaml:"performance"`
	Monitoring   MonitoringConfig   `yaml:"monitoring"`
	WebSocket    WebSocketConfig    `yaml:"websocket"`
	API          APIConfig          `yaml:"api"`
	Logging      AppLoggingConfig   `yaml:"logging"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port        string `yaml:"port"`
	UserStorage string `yaml:"user_storage"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Workspace WorkspaceCacheConfig `yaml:"workspace"`
}

type WorkspaceCacheConfig struct {
	MaxSize               int           `yaml:"max_size"`
	TTLMinutes            int           `yaml:"ttl_minutes"`
	CleanupIntervalMinutes int           `yaml:"cleanup_interval_minutes"`
	EnableStats           bool          `yaml:"enable_stats"`
}

// DeliveryConfig 投递系统配置
type DeliveryConfig struct {
	Workers WorkersConfig `yaml:"workers"`
	Queue   QueueConfig   `yaml:"queue"`
	Task    TaskConfig    `yaml:"task"`
}

type WorkersConfig struct {
	Count    int `yaml:"count"`
	MaxCount int `yaml:"max_count"`
	MinCount int `yaml:"min_count"`
}

type QueueConfig struct {
	EntrySize    int `yaml:"entry_size"`
	PrioritySize int `yaml:"priority_size"`
	WorkerSize   int `yaml:"worker_size"`
}

type TaskConfig struct {
	TimeoutSeconds     int `yaml:"timeout_seconds"`
	MaxRetries         int `yaml:"max_retries"`
	RetryBackoffBaseMs int `yaml:"retry_backoff_base_ms"`
	RetryBackoffMaxMs  int `yaml:"retry_backoff_max_ms"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	WAL      WALConfig      `yaml:"wal"`
	CacheSizeMessages   int `yaml:"cache_size_messages"`
	CacheSizeReadStatus int `yaml:"cache_size_read_status"`
}

type WALConfig struct {
	Enabled         bool   `yaml:"enabled"`
	SynchronousMode string `yaml:"synchronous_mode"`
}

// UserConfig 用户配置
type UserConfig struct {
	MaxMessagesPerDay int    `yaml:"max_messages_per_day"`
	MaxChannels       int    `yaml:"max_channels"`
	MessageSizeLimit  int64  `yaml:"message_size_limit"`
	MaxWorkspaces     int    `yaml:"max_workspaces"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	Backpressure BackpressureConfig `yaml:"backpressure"`
}

type BackpressureConfig struct {
	MemoryPressureThresholds MemoryPressureThresholds `yaml:"memory_pressure_thresholds"`
	RejectRates             RejectRates             `yaml:"reject_rates"`
}

type MemoryPressureThresholds struct {
	Critical float64 `yaml:"critical"`
	High     float64 `yaml:"high"`
	Medium   float64 `yaml:"medium"`
}

type RejectRates struct {
	CriticalPriority int `yaml:"critical_priority"`
	HighPriority     int `yaml:"high_priority"`
	MediumPriority   int `yaml:"medium_priority"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Metrics  MetricsConfig  `yaml:"metrics"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type MetricsConfig struct {
	EnablePrometheus bool `yaml:"enable_prometheus"`
	StatsLogInterval  int  `yaml:"stats_log_interval"`
}

type LoggingConfig struct {
	Level             string `yaml:"level"`
	EnableRequestLogs bool  `yaml:"enable_request_logs"`
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	MaxConnectionsPerUser int `yaml:"max_connections_per_user"`
	PingIntervalSeconds    int `yaml:"ping_interval_seconds"`
	ReadTimeoutSeconds    int `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds   int `yaml:"write_timeout_seconds"`
}

// APIConfig API配置
type APIConfig struct {
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	CORS      CORSConfig      `yaml:"cors"`
}

type RateLimitConfig struct {
	Enabled            bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	BurstSize         int  `yaml:"burst_size"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
}

// GetTTL 获取TTL时间间隔
func (c *CacheConfig) GetTTL() time.Duration {
	return time.Duration(c.Workspace.TTLMinutes) * time.Minute
}

// GetCleanupInterval 获取清理间隔
func (c *CacheConfig) GetCleanupInterval() time.Duration {
	return time.Duration(c.Workspace.CleanupIntervalMinutes) * time.Minute
}

// GetTaskTimeout 获取任务超时时间
func (d *DeliveryConfig) GetTaskTimeout() time.Duration {
	return time.Duration(d.Task.TimeoutSeconds) * time.Second
}

// GetRetryBackoffBase 获取重试退避基数
func (d *DeliveryConfig) GetRetryBackoffBase() time.Duration {
	return time.Duration(d.Task.RetryBackoffBaseMs) * time.Millisecond
}

// GetRetryBackoffMax 获取重试退避最大值
func (d *DeliveryConfig) GetRetryBackoffMax() time.Duration {
	return time.Duration(d.Task.RetryBackoffMaxMs) * time.Millisecond
}

// AppLoggingConfig 应用日志配置（重命名避免冲突）
type AppLoggingConfig struct {
	Level    string               `yaml:"level"`
	File     LogFileConfig        `yaml:"file"`
	Console  LogConsoleConfig     `yaml:"console"`
	Format   LogFormatConfig      `yaml:"format"`
}

// LogFileConfig 文件日志配置
type LogFileConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Path       string `yaml:"path"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
	Compress   bool   `yaml:"compress"`
}

// LogConsoleConfig 控制台日志配置
type LogConsoleConfig struct {
	Enabled bool `yaml:"enabled"`
	Color   bool `yaml:"color"`
}

// LogFormatConfig 日志格式配置
type LogFormatConfig struct {
	Timestamp   string `yaml:"timestamp"`
	Caller      bool   `yaml:"caller"`
	StackTrace  bool   `yaml:"stack_trace"`
}