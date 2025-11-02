package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 日志接口
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	Sync() error
}

// Config 日志配置
type Config struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	OutputPath string `json:"output_path"`
}

// zapLogger zap日志实现
type zapLogger struct {
	logger *zap.Logger
}

// NewLogger 创建新的日志记录器
func NewLogger(cfg Config) (Logger, error) {
	// 创建日志目录
	if cfg.OutputPath != "" {
		dir := filepath.Dir(cfg.OutputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// 解析日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 选择编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 创建输出
	var writers []zapcore.WriteSyncer
	// 默认输出到标准输出
	writers = append(writers, zapcore.AddSync(os.Stdout))

	// 如果指定了输出文件，则也输出到文件
	if cfg.OutputPath != "" {
		// 使用 lumberjack 进行日志轮转
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.OutputPath,
			MaxSize:    100, // megabytes
			MaxAge:     7,   // days
			MaxBackups: 10,
			LocalTime:  true,
			Compress:   true,
		}
		writers = append(writers, zapcore.AddSync(lumberjackLogger))
	}

	// 创建核心
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writers...), level),
	)

	// 创建logger
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &zapLogger{logger: logger}, nil
}

// Debug 记录调试日志
func (l *zapLogger) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg, l.toFields(fields...)...)
}

// Info 记录信息日志
func (l *zapLogger) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg, l.toFields(fields...)...)
}

// Warn 记录警告日志
func (l *zapLogger) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg, l.toFields(fields...)...)
}

// Error 记录错误日志
func (l *zapLogger) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg, l.toFields(fields...)...)
}

// Fatal 记录致命错误日志
func (l *zapLogger) Fatal(msg string, fields ...interface{}) {
	l.logger.Fatal(msg, l.toFields(fields...)...)
}

// Sync 同步日志缓冲区
func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}

// toFields 将字段转换为zap字段
func (l *zapLogger) toFields(fields ...interface{}) []zap.Field {
	if len(fields)%2 != 0 {
		// 如果字段数量不是偶数，添加一个空值
		fields = append(fields, "")
	}

	zapFields := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", fields[i])
		}

		value := fields[i+1]
		zapFields = append(zapFields, zap.Any(key, value))
	}

	return zapFields
}

// DefaultLogger 创建默认日志记录器
func DefaultLogger() Logger {
	logger, _ := NewLogger(Config{
		Level:  "info",
		Format: "console",
	})
	return logger
}
