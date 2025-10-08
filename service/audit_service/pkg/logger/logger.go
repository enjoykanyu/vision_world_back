package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path/filepath"
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
	Level      string
	Format     string
	OutputPath string
}

// zapLogger zap日志实现
type zapLogger struct {
	sugar *zap.SugaredLogger
}

// NewLogger 创建新的日志器
func NewLogger(cfg Config) (Logger, error) {
	// 确保日志目录存在
	if cfg.OutputPath != "" {
		dir := filepath.Dir(cfg.OutputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// 配置zap
	zapConfig := zap.NewProductionConfig()

	// 设置日志级别
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}
	zapConfig.Level = zap.NewAtomicLevelAt(level)

	// 设置输出格式
	if cfg.Format == "json" {
		zapConfig.Encoding = "json"
	} else {
		zapConfig.Encoding = "console"
	}

	// 设置输出位置
	if cfg.OutputPath != "" {
		zapConfig.OutputPaths = []string{cfg.OutputPath}
		zapConfig.ErrorOutputPaths = []string{cfg.OutputPath}
	} else {
		zapConfig.OutputPaths = []string{"stdout"}
		zapConfig.ErrorOutputPaths = []string{"stderr"}
	}

	// 创建logger
	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return &zapLogger{
		sugar: logger.Sugar(),
	}, nil
}

// Debug 调试日志
func (l *zapLogger) Debug(msg string, fields ...interface{}) {
	l.sugar.Debugw(msg, fields...)
}

// Info 信息日志
func (l *zapLogger) Info(msg string, fields ...interface{}) {
	l.sugar.Infow(msg, fields...)
}

// Warn 警告日志
func (l *zapLogger) Warn(msg string, fields ...interface{}) {
	l.sugar.Warnw(msg, fields...)
}

// Error 错误日志
func (l *zapLogger) Error(msg string, fields ...interface{}) {
	l.sugar.Errorw(msg, fields...)
}

// Fatal 致命错误日志
func (l *zapLogger) Fatal(msg string, fields ...interface{}) {
	l.sugar.Fatalw(msg, fields...)
}
