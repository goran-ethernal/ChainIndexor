package logger

import (
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var ValidLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

// root logger
var log atomic.Pointer[Logger]

// LoggingConfig interface to avoid circular dependency with pkg/config.
// Components will receive this interface instead of concrete config type.
type LoggingConfig interface {
	GetComponentLevel(component string) string
	GetDefaultLevel() string
	IsDevelopment() bool
}

// Logger wraps zap.SugaredLogger to provide a consistent logging interface across the project.
// It provides both structured logging (with fields) and printf-style logging methods.
type Logger struct {
	*zap.SugaredLogger
	atomicLevel zap.AtomicLevel
	component   string
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
	atomicLevel := zap.NewAtomicLevelAt(zapLevel)
	config.Level = atomicLevel

	// Build logger
	zapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{
		SugaredLogger: zapLogger.Sugar(),
		atomicLevel:   atomicLevel,
	}, nil
}

// NewComponentLogger creates a logger for a specific component with its own log level.
// This allows different components to have different log levels for granular control.
func NewComponentLogger(component string, level string, development bool) *Logger {
	logger, err := NewLogger(level, development)
	if err != nil {
		panic(err)
	}
	return logger.WithComponent(component)
}

// NewComponentLoggerFromConfig creates a logger for a component using the provided logging config.
// It uses the component-specific level if set, otherwise falls back to the default level.
func NewComponentLoggerFromConfig(component string, cfg LoggingConfig) *Logger {
	if cfg == nil {
		// No config provided, use default
		return NewComponentLogger(component, "info", false)
	}
	level := cfg.GetComponentLevel(component)
	return NewComponentLogger(component, level, cfg.IsDevelopment())
}

// NewNopLogger creates a no-op logger that discards all logs.
// Useful for testing.
func NewNopLogger() *Logger {
	return &Logger{
		SugaredLogger: zap.NewNop().Sugar(),
		atomicLevel:   zap.NewAtomicLevel(),
	}
}

// WithComponent creates a child logger with a component name field.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		SugaredLogger: l.With("component", component),
		atomicLevel:   l.atomicLevel,
		component:     component,
	}
}

// SetLevel changes the log level dynamically at runtime.
func (l *Logger) SetLevel(level string) error {
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return err
	}
	l.atomicLevel.SetLevel(zapLevel)
	return nil
}

// GetLevel returns the current log level as a string.
func (l *Logger) GetLevel() string {
	return l.atomicLevel.Level().String()
}

// GetComponent returns the component name if set.
func (l *Logger) GetComponent() string {
	return l.component
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
