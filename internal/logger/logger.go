package logger

import (
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// root logger
var log atomic.Pointer[Logger]

// Logger wraps zap.SugaredLogger to provide a consistent logging interface across the project.
// It provides both structured logging (with fields) and printf-style logging methods.
type Logger struct {
	*zap.SugaredLogger
}

// NewLogger creates a new logger with the specified configuration.
// level can be "debug", "info", "warn", "error"
// development mode enables stack traces and uses console encoder
func NewLogger(level string, development bool) (*Logger, error) {
	var config zap.Config

	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	// Parse log level
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, err
	}
	config.Level = zap.NewAtomicLevelAt(zapLevel)

	// Build logger
	zapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{SugaredLogger: zapLogger.Sugar()}, nil
}

// NewNopLogger creates a no-op logger that discards all logs.
// Useful for testing.
func NewNopLogger() *Logger {
	return &Logger{SugaredLogger: zap.NewNop().Sugar()}
}

// WithComponent creates a child logger with a component name field.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{SugaredLogger: l.With("component", component)}
}

// Close flushes any buffered log entries.
func (l *Logger) Close() error {
	return l.Sync()
}

func GetDefaultLogger() *Logger {
	l := log.Load()
	if l != nil {
		return l
	}
	// default level: debug
	zapLogger, err := NewLogger("debug", true)
	if err != nil {
		panic(err)
	}
	log.Store(zapLogger)
	return log.Load()
}
