package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
}

// Config 日志配置
type Config struct {
	Level      string `mapstructure:"level"`       // 日志级别: debug, info, warn, error
	Format     string `mapstructure:"format"`      // 输出格式: json, console
	OutputPath string `mapstructure:"output_path"` // 输出路径，为空则输出到控制台
}

// zapLogger zap日志实现
type zapLogger struct {
	logger *zap.Logger
}

// NewLogger 创建新的日志实例
func NewLogger(cfg Config) (Logger, error) {
	// 设置日志级别
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 设置输出
	var writeSyncer zapcore.WriteSyncer
	if cfg.OutputPath != "" {
		// 确保目录存在
		dir := filepath.Dir(cfg.OutputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// 创建日志文件
		file, err := os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		writeSyncer = zapcore.AddSync(file)
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// 创建核心
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 创建logger
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &zapLogger{
		logger: logger,
	}, nil
}

// Debug 调试日志
func (l *zapLogger) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, fieldsToZap(fields)...)
}

// Info 信息日志
func (l *zapLogger) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, fieldsToZap(fields)...)
}

// Warn 警告日志
func (l *zapLogger) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, fieldsToZap(fields)...)
}

// Error 错误日志
func (l *zapLogger) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, fieldsToZap(fields)...)
}

// Fatal 致命错误日志
func (l *zapLogger) Fatal(msg string, fields ...interface{}) {
	l.logger.Fatal(msg, fieldsToZap(fields)...)
}

// fieldsToZap 将字段转换为zap字段
func fieldsToZap(fields []interface{}) []zap.Field {
	if len(fields)%2 != 0 {
		return []zap.Field{zap.String("error", "fields must be key-value pairs")}
	}

	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			zapFields = append(zapFields, zap.String(fmt.Sprintf("field_%d", i), fmt.Sprintf("%v", fields[i])))
			continue
		}
		value := fields[i+1]
		zapFields = append(zapFields, zap.Any(key, value))
	}

	return zapFields
}
