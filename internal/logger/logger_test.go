package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		development bool
		wantErr     bool
	}{
		{
			name:        "debug level production",
			level:       "debug",
			development: false,
			wantErr:     false,
		},
		{
			name:        "info level production",
			level:       "info",
			development: false,
			wantErr:     false,
		},
		{
			name:        "warn level development",
			level:       "warn",
			development: true,
			wantErr:     false,
		},
		{
			name:        "error level development",
			level:       "error",
			development: true,
			wantErr:     false,
		},
		{
			name:        "invalid level",
			level:       "invalid",
			development: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.level, tt.development)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, logger)
			} else {
				require.NoError(t, err)
				require.NotNil(t, logger)
				require.NotNil(t, logger.SugaredLogger)
				require.Equal(t, tt.level, logger.GetLevel())
			}
		})
	}
}

func TestLogger_SetLevel(t *testing.T) {
	tests := []struct {
		name        string
		initialLvl  string
		newLevel    string
		wantErr     bool
		expectedLvl string
	}{
		{
			name:        "change from info to debug",
			initialLvl:  "info",
			newLevel:    "debug",
			wantErr:     false,
			expectedLvl: "debug",
		},
		{
			name:        "change from debug to error",
			initialLvl:  "debug",
			newLevel:    "error",
			wantErr:     false,
			expectedLvl: "error",
		},
		{
			name:        "change from warn to info",
			initialLvl:  "warn",
			newLevel:    "info",
			wantErr:     false,
			expectedLvl: "info",
		},
		{
			name:       "invalid level",
			initialLvl: "info",
			newLevel:   "invalid",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.initialLvl, false)
			require.NoError(t, err)
			require.Equal(t, tt.initialLvl, logger.GetLevel())

			err = logger.SetLevel(tt.newLevel)
			if tt.wantErr {
				require.Error(t, err)
				// Level should remain unchanged on error
				require.Equal(t, tt.initialLvl, logger.GetLevel())
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLvl, logger.GetLevel())
			}
		})
	}
}

func TestLogger_WithComponent(t *testing.T) {
	logger, err := NewLogger("info", false)
	require.NoError(t, err)

	componentLogger := logger.WithComponent("test-component")
	require.NotNil(t, componentLogger)
	require.Equal(t, "test-component", componentLogger.GetComponent())

	// Should share the same atomic level
	require.Equal(t, logger.GetLevel(), componentLogger.GetLevel())

	// Changing level on parent should affect child
	err = logger.SetLevel("debug")
	require.NoError(t, err)
	require.Equal(t, "debug", componentLogger.GetLevel())
}

func TestNewComponentLogger(t *testing.T) {
	tests := []struct {
		name        string
		component   string
		level       string
		development bool
		wantErr     bool
	}{
		{
			name:        "valid component logger",
			component:   "downloader",
			level:       "info",
			development: false,
			wantErr:     false,
		},
		{
			name:        "debug level component",
			component:   "log-fetcher",
			level:       "debug",
			development: true,
			wantErr:     false,
		},
		{
			name:        "invalid level",
			component:   "sync-manager",
			level:       "invalid",
			development: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				require.Panics(t, func() {
					_ = NewComponentLogger(tt.component, tt.level, tt.development)
				})
			} else {
				logger := NewComponentLogger(tt.component, tt.level, tt.development)
				require.NotNil(t, logger)
				require.Equal(t, tt.component, logger.GetComponent())
				require.Equal(t, tt.level, logger.GetLevel())
			}
		})
	}
}

func TestNewNopLogger(t *testing.T) {
	logger := NewNopLogger()
	require.NotNil(t, logger)
	require.NotNil(t, logger.SugaredLogger)

	// Nop logger should not panic on any log call
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")
}

func TestLogger_GetComponent(t *testing.T) {
	logger, err := NewLogger("info", false)
	require.NoError(t, err)

	// Logger without component should return empty string
	require.Equal(t, "", logger.GetComponent())

	// Logger with component
	componentLogger := logger.WithComponent("test-component")
	require.Equal(t, "test-component", componentLogger.GetComponent())
}

// mockLoggingConfig implements the LoggingConfig interface for testing
type mockLoggingConfig struct {
	defaultLevel    string
	development     bool
	componentLevels map[string]string
}

func (m *mockLoggingConfig) GetComponentLevel(component string) string {
	if level, ok := m.componentLevels[component]; ok {
		return level
	}
	return m.defaultLevel
}

func (m *mockLoggingConfig) GetDefaultLevel() string {
	return m.defaultLevel
}

func (m *mockLoggingConfig) IsDevelopment() bool {
	return m.development
}

func TestNewComponentLoggerFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		component     string
		config        LoggingConfig
		expectedLevel string
		wantErr       bool
	}{
		{
			name:      "component with specific level",
			component: "downloader",
			config: &mockLoggingConfig{
				defaultLevel: "info",
				development:  false,
				componentLevels: map[string]string{
					"downloader": "debug",
				},
			},
			expectedLevel: "debug",
			wantErr:       false,
		},
		{
			name:      "component using default level",
			component: "sync-manager",
			config: &mockLoggingConfig{
				defaultLevel:    "warn",
				development:     false,
				componentLevels: map[string]string{},
			},
			expectedLevel: "warn",
			wantErr:       false,
		},
		{
			name:      "development mode enabled",
			component: "log-fetcher",
			config: &mockLoggingConfig{
				defaultLevel: "debug",
				development:  true,
				componentLevels: map[string]string{
					"log-fetcher": "debug",
				},
			},
			expectedLevel: "debug",
			wantErr:       false,
		},
		{
			name:          "nil config uses defaults",
			component:     "maintenance",
			config:        nil,
			expectedLevel: "info",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewComponentLoggerFromConfig(tt.component, tt.config)
			if tt.wantErr {
				require.Panics(t, func() {
					_ = NewComponentLoggerFromConfig(tt.component, tt.config)
				})
			} else {
				require.NotNil(t, logger)
				require.Equal(t, tt.component, logger.GetComponent())
				require.Equal(t, tt.expectedLevel, logger.GetLevel())
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	// Create logger with warn level
	logger, err := NewLogger("warn", false)
	require.NoError(t, err)

	// Check that the atomic level is correctly configured
	require.False(t, logger.atomicLevel.Enabled(zapcore.DebugLevel))
	require.False(t, logger.atomicLevel.Enabled(zapcore.InfoLevel))
	require.True(t, logger.atomicLevel.Enabled(zapcore.WarnLevel))
	require.True(t, logger.atomicLevel.Enabled(zapcore.ErrorLevel))

	// Change to debug level
	err = logger.SetLevel("debug")
	require.NoError(t, err)

	// Now all levels should be enabled
	require.True(t, logger.atomicLevel.Enabled(zapcore.DebugLevel))
	require.True(t, logger.atomicLevel.Enabled(zapcore.InfoLevel))
	require.True(t, logger.atomicLevel.Enabled(zapcore.WarnLevel))
	require.True(t, logger.atomicLevel.Enabled(zapcore.ErrorLevel))
}

func TestLogger_MultipleComponents(t *testing.T) {
	// Create base logger
	baseLogger, err := NewLogger("info", false)
	require.NoError(t, err)

	// Create multiple component loggers
	downloader := baseLogger.WithComponent("downloader")
	fetcher := baseLogger.WithComponent("log-fetcher")
	store := baseLogger.WithComponent("log-store")

	// All should share the same level
	require.Equal(t, "info", downloader.GetLevel())
	require.Equal(t, "info", fetcher.GetLevel())
	require.Equal(t, "info", store.GetLevel())

	// But have different component names
	require.Equal(t, "downloader", downloader.GetComponent())
	require.Equal(t, "log-fetcher", fetcher.GetComponent())
	require.Equal(t, "log-store", store.GetComponent())

	// Changing base logger level affects all
	err = baseLogger.SetLevel("debug")
	require.NoError(t, err)
	require.Equal(t, "debug", downloader.GetLevel())
	require.Equal(t, "debug", fetcher.GetLevel())
	require.Equal(t, "debug", store.GetLevel())
}
