package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

func InitLogger(level string, logFile string) {
	// 确保日志目录存在
	if logFile != "" {
		dir := filepath.Dir(logFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			panic("Failed to create log directory: " + err.Error())
		}
	}

	// 日志级别
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// 编码器配置
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

	// 控制台输出
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleWriter := zapcore.AddSync(os.Stdout)

	// 文件输出
	var fileWriter zapcore.WriteSyncer
	if logFile != "" {
		fileWriter = zapcore.AddSync(&lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    100, // MB
			MaxBackups: 3,
			MaxAge:     7, // days
			Compress:   true,
		})
	}

	// 创建核心
	var core zapcore.Core
	if logFile != "" {
		// 同时输出到控制台和文件
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		core = zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, consoleWriter, zapLevel),
			zapcore.NewCore(fileEncoder, fileWriter, zapLevel),
		)
	} else {
		// 只输出到控制台
		core = zapcore.NewCore(consoleEncoder, consoleWriter, zapLevel)
	}

	// 创建logger
	logger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
}

func Debug(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Debug(msg, fields...)
	}
}

func Info(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Info(msg, fields...)
	}
}

func Warn(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Warn(msg, fields...)
	}
}

func Error(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Error(msg, fields...)
	}
}

func Fatal(msg string, fields ...zap.Field) {
	if logger != nil {
		logger.Fatal(msg, fields...)
	}
}

func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}
