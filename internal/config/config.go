package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// 配置相关常量
const (
	DefaultConfigFile      = "./data/config/config.yaml"
	EnvConfigFile         = "MIEMIE_CONFIG_FILE"
	DefaultPort            = "8080"
	DefaultUserStorage     = "./data/user"
	DefaultMaxSize          = 1000
	DefaultTTLMinutes       = 30
	DefaultCleanupMinutes    = 5
)

// Load 加载配置文件
func Load() (*Config, error) {
	// 获取配置文件路径
	configFile := getConfigFile()

	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// 配置文件不存在，创建默认配置
		if err := createDefaultConfig(configFile); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		fmt.Printf("Created default config file: %s\n", configFile)
	}

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Reload 重新加载配置
func Reload(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	setDefaults(&config)

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// getConfigFile 获取配置文件路径
func getConfigFile() string {
	if envFile := os.Getenv(EnvConfigFile); envFile != "" {
		return envFile
	}
	return DefaultConfigFile
}

// createDefaultConfig 创建默认配置文件
func createDefaultConfig(configFile string) error {
	// 确保目录存在
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// 创建默认配置内容
	defaultConfig := `# Mienie 消息系统配置文件
# 版本: v1.0

# 服务器配置
server:
  port: "8080"
  user_storage: "./data/user"

# 缓存配置
cache:
  workspace:
    max_size: 1000              # 最大缓存工作空间数
    ttl_minutes: 30              # 缓存过期时间(分钟)
    cleanup_interval_minutes: 5 # 清理间隔(分钟)
    enable_stats: true           # 是否启用统计

# 投递系统配置
delivery:
  workers:
    count: 4                     # 邮递员数量(0=自动检测CPU核心数)
    max_count: 8                 # 最大邮递员数
    min_count: 2                 # 最小邮递员数
  queue:
    entry_size: 10000            # 入口队列大小
    priority_size: 3333          # 优先级队列大小
    worker_size: 100             # 工作队列大小
  task:
    timeout_seconds: 30          # 任务超时时间(秒)
    max_retries: 3               # 最大重试次数
    retry_backoff_base_ms: 100   # 重试退避基数(毫秒)
    retry_backoff_max_ms: 5000   # 重试退避最大值(毫秒)

# 数据库配置
database:
  wal:
    enabled: true                # 启用WAL模式
    synchronous_mode: "NORMAL"  # 同步模式: OFF/NORMAL/FULL
  cache_size_messages: 10000     # 消息数据库缓存大小(页)
  cache_size_read_status: 5000  # 已读状态数据库缓存大小(页)

# 用户限制
user:
  max_messages_per_day: 10000   # 每用户每天最大消息数
  max_channels: 50             # 每用户最大频道数
  message_size_limit: 1048576  # 单条消息大小限制(1MB)
  max_workspaces: 2000         # 系统最大工作空间数

# 性能调优
performance:
  backpressure:
    memory_pressure_thresholds: # 内存压力阈值
      critical: 0.9            # 90%
      high: 0.7                # 70%
      medium: 0.5              # 50%
    reject_rates:              # 拒绝率阈值
      critical_priority: 8    # 只接受8-10优先级
      high_priority: 6        # 拒绝率30%时接受6+
      medium_priority: 4      # 拒绝率50%时接受4+

# 监控和日志
monitoring:
  metrics:
    enable_prometheus: false     # 启用Prometheus指标
    stats_log_interval: 10      # 统计日志间隔(秒)
  logging:
    level: "info"               # 日志级别: debug/info/warn/error
    enable_request_logs: false   # 启用请求日志

# WebSocket配置
websocket:
  max_connections_per_user: 10  # 每用户最大WebSocket连接数
  ping_interval_seconds: 30     # 心跳间隔(秒)
  read_timeout_seconds: 60      # 读取超时(秒)
  write_timeout_seconds: 10     # 写入超时(秒)

# API配置
api:
  rate_limit:
    enabled: true               # 启用速率限制
    requests_per_minute: 1000    # 每分钟请求限制
    burst_size: 100             # 突发请求大小
  cors:
    enabled: true               # 启用CORS
    allowed_origins: ["*"]      # 允许的源
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["Content-Type", "Authorization", "User-ID"]

# 日志配置
logging:
  level: "info"                 # 日志级别: debug/info/warn/error
  file:
    enabled: true               # 启用文件日志
    path: "./data/logs/application.log"  # 日志文件路径
    max_size_mb: 50             # 单个文件最大大小(MB)
    max_backups: 10             # 保留历史文件数量
    max_age_days: 7             # 文件最大保留天数
    compress: false             # 是否压缩历史文件
  console:
    enabled: true               # 启用控制台输出
    color: true                 # 是否彩色输出
  format:
    timestamp: "2006-01-02 15:04:05.000"  # 时间格式
    caller: true                # 是否显示调用者信息
    stack_trace: true           # 是否显示堆栈跟踪
`

	return os.WriteFile(configFile, []byte(defaultConfig), 0644)
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	// 服务器默认值
	if config.Server.Port == "" {
		config.Server.Port = DefaultPort
	}
	if config.Server.UserStorage == "" {
		config.Server.UserStorage = DefaultUserStorage
	}

	// 缓存默认值
	if config.Cache.Workspace.MaxSize == 0 {
		config.Cache.Workspace.MaxSize = DefaultMaxSize
	}
	if config.Cache.Workspace.TTLMinutes == 0 {
		config.Cache.Workspace.TTLMinutes = DefaultTTLMinutes
	}
	if config.Cache.Workspace.CleanupIntervalMinutes == 0 {
		config.Cache.Workspace.CleanupIntervalMinutes = DefaultCleanupMinutes
	}

	// 日志默认值
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.File.Path == "" {
		config.Logging.File.Path = "./data/logs/application.log"
	}
	if config.Logging.File.MaxSizeMB == 0 {
		config.Logging.File.MaxSizeMB = 50
	}
	if config.Logging.File.MaxBackups == 0 {
		config.Logging.File.MaxBackups = 10
	}
	if config.Logging.File.MaxAgeDays == 0 {
		config.Logging.File.MaxAgeDays = 7
	}
	if config.Logging.Format.Timestamp == "" {
		config.Logging.Format.Timestamp = "2006-01-02 15:04:05.000"
	}
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	// 验证端口
	port := config.Server.Port
	if port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	// 验证缓存配置
	if config.Cache.Workspace.MaxSize <= 0 {
		return fmt.Errorf("cache max_size must be positive")
	}
	if config.Cache.Workspace.TTLMinutes <= 0 {
		return fmt.Errorf("cache ttl_minutes must be positive")
	}

	// 验证投递系统配置
	if config.Delivery.Workers.Count < 0 {
		return fmt.Errorf("delivery workers count cannot be negative")
	}
	if config.Delivery.Workers.MaxCount < config.Delivery.Workers.MinCount {
		return fmt.Errorf("delivery workers max_count must be >= min_count")
	}
	if config.Delivery.Task.TimeoutSeconds <= 0 {
		return fmt.Errorf("delivery task timeout must be positive")
	}

	// 验证用户配置
	if config.User.MaxMessagesPerDay <= 0 {
		return fmt.Errorf("user max_messages_per_day must be positive")
	}
	if config.User.MessageSizeLimit <= 0 {
		return fmt.Errorf("user message_size_limit must be positive")
	}

	return nil
}

// GetEnv 获取环境变量（向后兼容）
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}