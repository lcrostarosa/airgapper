// Package logging provides structured logging using zap
package logging

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger
	once   sync.Once
)

// Config holds logging configuration
type Config struct {
	Level       string // debug, info, warn, error
	Development bool   // enables development mode (more verbose)
	JSON        bool   // output as JSON (for production)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Level:       "info",
		Development: false,
		JSON:        false,
	}
}

// Init initializes the global logger
func Init(cfg Config) error {
	var err error
	once.Do(func() {
		err = initLogger(cfg)
	})
	return err
}

func initLogger(cfg Config) error {
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	var zapCfg zap.Config
	if cfg.Development {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapCfg = zap.NewProductionConfig()
		if !cfg.JSON {
			zapCfg.Encoding = "console"
			zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		}
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)

	var err error
	logger, err = zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	sugar = logger.Sugar()
	return nil
}

// InitDefault initializes with default configuration
func InitDefault() {
	if logger == nil {
		_ = Init(DefaultConfig())
	}
}

// L returns the global logger
func L() *zap.Logger {
	InitDefault()
	return logger
}

// S returns the global sugared logger
func S() *zap.SugaredLogger {
	InitDefault()
	return sugar
}

// Sync flushes any buffered log entries
func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}

// --- Convenience functions ---

// Debug logs a debug message with fields
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// Info logs an info message with fields
func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// Warn logs a warning message with fields
func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// Error logs an error message with fields
func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}

// --- Sugared convenience functions (printf-style) ---

// Debugf logs a formatted debug message
func Debugf(template string, args ...interface{}) {
	S().Debugf(template, args...)
}

// Infof logs a formatted info message
func Infof(template string, args ...interface{}) {
	S().Infof(template, args...)
}

// Warnf logs a formatted warning message
func Warnf(template string, args ...interface{}) {
	S().Warnf(template, args...)
}

// Errorf logs a formatted error message
func Errorf(template string, args ...interface{}) {
	S().Errorf(template, args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(template string, args ...interface{}) {
	S().Fatalf(template, args...)
}

// --- Field constructors for common types ---

// String creates a string field
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

// Int creates an int field
func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

// Int64 creates an int64 field
func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

// Bool creates a bool field
func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

// Err creates an error field
func Err(err error) zap.Field {
	return zap.Error(err)
}

// Any creates a field for any type
func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

// Duration creates a duration field
func Duration(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

// --- Writer adapter for http server ---

// WriterAdapter adapts the logger for use with http.Server.ErrorLog
type WriterAdapter struct{}

func (w WriterAdapter) Write(p []byte) (n int, err error) {
	// Trim trailing newline
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	Error(msg)
	return len(p), nil
}

// NewWriterAdapter returns a writer that logs errors
func NewWriterAdapter() *WriterAdapter {
	return &WriterAdapter{}
}

// StdLogger returns a standard library *log.Logger that writes to zap
func StdLogger() *stdLoggerAdapter {
	return &stdLoggerAdapter{}
}

type stdLoggerAdapter struct{}

func (l *stdLoggerAdapter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	Info(msg)
	return os.Stdout.Write(p)
}
