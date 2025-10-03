package logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

var (
	Logger *zap.Logger
	sugar  *zap.SugaredLogger
)

// InitLogger 初始化日志
func InitLogger(cfg LogConfig) error {
	var err error
	Logger, err = newZapLogger(cfg)
	if err != nil {
		return fmt.Errorf("初始化日志失败: %v", err)
	}
	sugar = Logger.Sugar()
	return nil
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string
	Format     string
	Output     string
	FilePath   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

// newZapLogger 创建zap日志
func newZapLogger(cfg LogConfig) (*zap.Logger, error) {
	// 日志级别
	level := zap.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn", "warning":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	case "fatal":
		level = zap.FatalLevel
	}

	// 编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 输出方式
	var writeSyncer zapcore.WriteSyncer
	switch cfg.Output {
	case "file":
		// 文件输出
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %v", err)
		}
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		})
	case "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// 创建Core
	core := zapcore.NewCore(
		getEncoder(cfg.Format, encoderConfig),
		writeSyncer,
		level,
	)

	// 创建Logger
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)), nil
}

// getEncoder 获取编码器
func getEncoder(format string, encoderConfig zapcore.EncoderConfig) zapcore.Encoder {
	if format == "json" {
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// customTimeEncoder 自定义时间编码器
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// lumberjack.Logger 兼容类型
type lumberjackLogger interface {
	Write(p []byte) (n int, err error)
	Close() error
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Fatal 致命错误日志
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// Debugf 格式化调试日志
func Debugf(template string, args ...interface{}) {
	sugar.Debugf(template, args...)
}

// Infof 格式化信息日志
func Infof(template string, args ...interface{}) {
	sugar.Infof(template, args...)
}

// Warnf 格式化警告日志
func Warnf(template string, args ...interface{}) {
	sugar.Warnf(template, args...)
}

// Errorf 格式化错误日志
func Errorf(template string, args ...interface{}) {
	sugar.Errorf(template, args...)
}

// Fatalf 格式化致命错误日志
func Fatalf(template string, args ...interface{}) {
	sugar.Fatalf(template, args...)
}

// Debugw 结构化调试日志
func Debugw(msg string, keysAndValues ...interface{}) {
	sugar.Debugw(msg, keysAndValues...)
}

// Infow 结构化信息日志
func Infow(msg string, keysAndValues ...interface{}) {
	sugar.Infow(msg, keysAndValues...)
}

// Warnw 结构化警告日志
func Warnw(msg string, keysAndValues ...interface{}) {
	sugar.Warnw(msg, keysAndValues...)
}

// Errorw 结构化错误日志
func Errorw(msg string, keysAndValues ...interface{}) {
	sugar.Errorw(msg, keysAndValues...)
}

// Fatalw 结构化致命错误日志
func Fatalw(msg string, keysAndValues ...interface{}) {
	sugar.Fatalw(msg, keysAndValues...)
}

// WithContext 添加上下文信息
func WithContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Logger
	}

	// 从上下文中获取追踪信息
	traceID := ctx.Value("trace_id")
	spanID := ctx.Value("span_id")
	userID := ctx.Value("user_id")

	fields := []zap.Field{}
	if traceID != nil {
		fields = append(fields, zap.Any("trace_id", traceID))
	}
	if spanID != nil {
		fields = append(fields, zap.Any("span_id", spanID))
	}
	if userID != nil {
		fields = append(fields, zap.Any("user_id", userID))
	}

	return Logger.With(fields...)
}

// Sync 同步日志
func Sync() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}

// NewGormLogger 创建GORM日志
func NewGormLogger(level string) logger.Interface {
	logLevel := logger.Info
	switch strings.ToLower(level) {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn", "warning":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	}

	return &gormLogger{
		level:                 logLevel,
		SlowThreshold:         200 * time.Millisecond,
		SkipErrRecordNotFound: true,
		ParameterizedQueries:  true,
	}
}

// gormLogger GORM日志实现
type gormLogger struct {
	level                 logger.LogLevel
	SlowThreshold         time.Duration
	SkipErrRecordNotFound bool
	ParameterizedQueries  bool
}

func (l *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := *l
	newlogger.level = level
	return &newlogger
}

func (l *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= logger.Info {
		Infow(fmt.Sprintf(msg, data...), "source", "gorm")
	}
}

func (l *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= logger.Warn {
		Warnw(fmt.Sprintf(msg, data...), "source", "gorm")
	}
}

func (l *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= logger.Error {
		Errorw(fmt.Sprintf(msg, data...), "source", "gorm")
	}
}

func (l *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.level >= logger.Error && (!l.SkipErrRecordNotFound || !errors.Is(err, logger.ErrRecordNotFound)):
		sql, rows := fc()
		Errorw("SQL执行错误",
			"source", "gorm",
			"error", err,
			"elapsed", elapsed,
			"sql", sql,
			"rows", rows,
			"file", utils.FileWithLineNum(),
		)
	case l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.level >= logger.Warn:
		sql, rows := fc()
		Warnw("慢查询",
			"source", "gorm",
			"elapsed", elapsed,
			"sql", sql,
			"rows", rows,
			"file", utils.FileWithLineNum(),
		)
	case l.level >= logger.Info:
		sql, rows := fc()
		Infow("SQL执行",
			"source", "gorm",
			"elapsed", elapsed,
			"sql", sql,
			"rows", rows,
			"file", utils.FileWithLineNum(),
		)
	}
}
