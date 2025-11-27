package logger

import (
	"io"
	"os"
	"path/filepath"
	"miemie/internal/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 全局日志器实例
var Logger *logrus.Logger

// InitLogger 初始化日志系统
func InitLogger(cfg *config.AppLoggingConfig) error {
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel // 默认级别
	}
	Logger.SetLevel(level)

	// 设置日志格式 - 简化格式，不显示调用者信息
	if cfg.Format.Caller {
		// 只在debug模式下显示调用者信息
		if cfg.Level == "debug" {
			Logger.SetReportCaller(true)
		}
	}

	// 创建多输出写入器
	var writers []io.Writer

	// 控制台输出
	if cfg.Console.Enabled {
		consoleWriter := os.Stdout
		if cfg.Console.Color {
			// 彩色输出使用完整的日期时间格式
			Logger.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006-01-02 15:04:05", // 完整日期时间格式
				DisableQuote:    true,      // 不加引号
			})
		} else {
			// 无颜色输出
			Logger.SetFormatter(&logrus.TextFormatter{
				FullTimestamp:   true,
				TimestampFormat: "2006-01-02 15:04:05",
				DisableColors:   true,
				DisableQuote:    true,
			})
		}
		writers = append(writers, consoleWriter)
	}

	// 文件输出
	if cfg.File.Enabled {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.File.Path)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 配置 lumberjack 文件回滚
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSizeMB,    // MB
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAgeDays,   // days
			Compress:   cfg.File.Compress,
		}

		writers = append(writers, fileWriter)

		// 文件日志使用简化的 JSON 格式
		if !cfg.Console.Enabled {
			// 如果只有文件输出，使用JSON格式
			Logger.SetFormatter(&logrus.JSONFormatter{
				TimestampFormat: "2006-01-02 15:04:05", // 简化时间格式
				FieldMap: logrus.FieldMap{
					logrus.FieldKeyTime:  "timestamp",
					logrus.FieldKeyLevel: "level",
					logrus.FieldKeyMsg:   "message",
				},
			})
		}
	}

	// 设置多输出
	if len(writers) > 0 {
		Logger.SetOutput(io.MultiWriter(writers...))
	}

	// 记录日志系统启动信息
	Logger.Info("Logger initialized successfully")
	Logger.Infof("Log level: %s", level.String())
	if cfg.File.Enabled {
		Logger.Infof("File logging enabled: %s", cfg.File.Path)
	}
	if cfg.Console.Enabled {
		Logger.Info("Console logging enabled")
	}

	return nil
}

// GetLogger 获取日志器实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		// 如果未初始化，返回默认日志器
		Logger = logrus.New()
		Logger.SetLevel(logrus.InfoLevel)
	}
	return Logger
}

// Debug 记录调试信息
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Info 记录一般信息
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Warn 记录警告信息
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Error 记录错误信息
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Fatal 记录致命错误并退出
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Debugf 记录格式化的调试信息
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Infof 记录格式化的一般信息
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warnf 记录格式化的警告信息
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Errorf 记录格式化的错误信息
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatalf 记录格式化的致命错误并退出
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// WithField 添加字段
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields 添加多个字段
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// WithError 添加错误字段
func WithError(err error) *logrus.Entry {
	return GetLogger().WithError(err)
}