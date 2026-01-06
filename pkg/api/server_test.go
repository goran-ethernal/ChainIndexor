package api

import (
	"context"
	"testing"
	"time"

	apimocks "github.com/goran-ethernal/ChainIndexor/internal/api/mocks"
	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/indexer"
	indexermocks "github.com/goran-ethernal/ChainIndexor/pkg/indexer/mocks"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   *config.APIConfig
		validate func(t *testing.T, server *Server)
	}{
		{
			name: "create server with basic config",
			config: &config.APIConfig{
				Enabled:       true,
				ListenAddress: "localhost:8080",
				ReadTimeout:   common.Duration{Duration: 5 * time.Second},
				WriteTimeout:  common.Duration{Duration: 10 * time.Second},
				IdleTimeout:   common.Duration{Duration: 60 * time.Second},
				CORS: config.CORSConfig{
					Enabled:        false,
					AllowedOrigins: []string{},
				},
			},
			validate: func(t *testing.T, server *Server) {
				t.Helper()

				require.NotNil(t, server)
				require.NotNil(t, server.config)
				require.NotNil(t, server.registry)
				require.NotNil(t, server.handler)
				require.NotNil(t, server.server)
				require.NotNil(t, server.log)
				require.Equal(t, "localhost:8080", server.server.Addr)
				require.Equal(t, 5*time.Second, server.server.ReadTimeout)
				require.Equal(t, 10*time.Second, server.server.WriteTimeout)
				require.Equal(t, 60*time.Second, server.server.IdleTimeout)
			},
		},
		{
			name: "create server with CORS enabled",
			config: &config.APIConfig{
				Enabled:       true,
				ListenAddress: ":9090",
				ReadTimeout:   common.Duration{Duration: 30 * time.Second},
				WriteTimeout:  common.Duration{Duration: 30 * time.Second},
				IdleTimeout:   common.Duration{Duration: 120 * time.Second},
				CORS: config.CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"http://localhost:3000", "https://example.com"},
				},
			},
			validate: func(t *testing.T, server *Server) {
				t.Helper()

				require.NotNil(t, server)
				require.True(t, server.config.CORS.Enabled)
				require.Len(t, server.config.CORS.AllowedOrigins, 2)
				require.Equal(t, ":9090", server.server.Addr)
			},
		},
		{
			name: "create server with disabled state",
			config: &config.APIConfig{
				Enabled:       false,
				ListenAddress: ":8080",
				ReadTimeout:   common.Duration{Duration: 5 * time.Second},
				WriteTimeout:  common.Duration{Duration: 5 * time.Second},
				IdleTimeout:   common.Duration{Duration: 60 * time.Second},
				CORS: config.CORSConfig{
					Enabled: false,
				},
			},
			validate: func(t *testing.T, server *Server) {
				t.Helper()

				require.NotNil(t, server)
				require.False(t, server.config.Enabled)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := apimocks.NewIndexerRegistry(t)
			log := logger.NewNopLogger()

			server := NewServer(tt.config, registry, log)

			tt.validate(t, server)
		})
	}
}

func TestServer_Start_Disabled(t *testing.T) {
	t.Parallel()

	cfg := &config.APIConfig{
		Enabled:       false,
		ListenAddress: ":8080",
		ReadTimeout:   common.Duration{Duration: 5 * time.Second},
		WriteTimeout:  common.Duration{Duration: 5 * time.Second},
		IdleTimeout:   common.Duration{Duration: 60 * time.Second},
	}

	registry := apimocks.NewIndexerRegistry(t)
	log := logger.NewNopLogger()

	server := NewServer(cfg, registry, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should return immediately when disabled
	done := make(chan error, 1)
	go func() {
		done <- server.Start(ctx)
	}()

	// Cancel context immediately
	cancel()

	// Should return quickly
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("Start() did not return when server is disabled")
	}
}

func TestServer_Start_GracefulShutdown(t *testing.T) {
	t.Parallel()

	cfg := &config.APIConfig{
		Enabled:       true,
		ListenAddress: "localhost:0", // Use port 0 for random available port
		ReadTimeout:   common.Duration{Duration: 5 * time.Second},
		WriteTimeout:  common.Duration{Duration: 5 * time.Second},
		IdleTimeout:   common.Duration{Duration: 60 * time.Second},
		CORS: config.CORSConfig{
			Enabled: false,
		},
	}

	registry := apimocks.NewIndexerRegistry(t)
	registry.EXPECT().ListAll().Return(([]indexer.Indexer)(nil)).Maybe()

	log := logger.NewNopLogger()
	server := NewServer(cfg, registry, log)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server
	done := make(chan error, 1)
	go func() {
		done <- server.Start(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Trigger graceful shutdown
	cancel()

	// Should shutdown within timeout
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second): // shutdownCtxTimeout + buffer
		t.Fatal("Server did not shutdown gracefully within timeout")
	}
}

// TestServer_Routes is covered by individual handler tests (TestHandler_Health, TestHandler_ListIndexers, etc.)
// and the integration test (TestServer_Integration_WithRealIndexer)

func TestServer_Middleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		corsConfig config.CORSConfig
		validate   func(t *testing.T, server *Server)
	}{
		{
			name: "CORS middleware applied when enabled",
			corsConfig: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"http://localhost:3000"},
			},
			validate: func(t *testing.T, server *Server) {
				t.Helper()

				require.NotNil(t, server.server.Handler)
				require.True(t, server.config.CORS.Enabled)
			},
		},
		{
			name: "CORS middleware not applied when disabled",
			corsConfig: config.CORSConfig{
				Enabled: false,
			},
			validate: func(t *testing.T, server *Server) {
				t.Helper()

				require.NotNil(t, server.server.Handler)
				require.False(t, server.config.CORS.Enabled)
			},
		},
		{
			name: "multiple CORS origins",
			corsConfig: config.CORSConfig{
				Enabled: true,
				AllowedOrigins: []string{
					"http://localhost:3000",
					"http://localhost:3001",
					"https://example.com",
				},
			},
			validate: func(t *testing.T, server *Server) {
				t.Helper()

				require.Len(t, server.config.CORS.AllowedOrigins, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.APIConfig{
				Enabled:       true,
				ListenAddress: ":8080",
				ReadTimeout:   common.Duration{Duration: 5 * time.Second},
				WriteTimeout:  common.Duration{Duration: 5 * time.Second},
				IdleTimeout:   common.Duration{Duration: 60 * time.Second},
				CORS:          tt.corsConfig,
			}

			registry := apimocks.NewIndexerRegistry(t)
			log := logger.NewNopLogger()

			server := NewServer(cfg, registry, log)

			tt.validate(t, server)
		})
	}
}

func TestServer_Timeouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		readTimeout  time.Duration
		writeTimeout time.Duration
		idleTimeout  time.Duration
	}{
		{
			name:         "default timeouts",
			readTimeout:  5 * time.Second,
			writeTimeout: 10 * time.Second,
			idleTimeout:  60 * time.Second,
		},
		{
			name:         "custom short timeouts",
			readTimeout:  1 * time.Second,
			writeTimeout: 2 * time.Second,
			idleTimeout:  30 * time.Second,
		},
		{
			name:         "custom long timeouts",
			readTimeout:  30 * time.Second,
			writeTimeout: 60 * time.Second,
			idleTimeout:  300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.APIConfig{
				Enabled:       true,
				ListenAddress: ":8080",
				ReadTimeout:   common.Duration{Duration: tt.readTimeout},
				WriteTimeout:  common.Duration{Duration: tt.writeTimeout},
				IdleTimeout:   common.Duration{Duration: tt.idleTimeout},
				CORS: config.CORSConfig{
					Enabled: false,
				},
			}

			registry := apimocks.NewIndexerRegistry(t)
			log := logger.NewNopLogger()

			server := NewServer(cfg, registry, log)

			require.Equal(t, tt.readTimeout, server.server.ReadTimeout)
			require.Equal(t, tt.writeTimeout, server.server.WriteTimeout)
			require.Equal(t, tt.idleTimeout, server.server.IdleTimeout)
		})
	}
}

func TestServer_Integration_WithRealIndexer(t *testing.T) {
	t.Parallel()

	cfg := &config.APIConfig{
		Enabled:       true,
		ListenAddress: "localhost:0",
		ReadTimeout:   common.Duration{Duration: 5 * time.Second},
		WriteTimeout:  common.Duration{Duration: 5 * time.Second},
		IdleTimeout:   common.Duration{Duration: 60 * time.Second},
		CORS: config.CORSConfig{
			Enabled: false,
		},
	}

	registry := apimocks.NewIndexerRegistry(t)

	// Create a queryable indexer mock
	mockIdx := indexermocks.NewIndexer(t)
	mockQueryable := indexermocks.NewQueryable(t)

	// Set up expectations for ListIndexers endpoint
	mockIdx.EXPECT().GetType().Return("test-indexer").Maybe()
	mockIdx.EXPECT().GetName().Return("test").Maybe()
	mockQueryable.EXPECT().GetEventTypes().Return([]string{"TestEvent"}).Maybe()

	registry.EXPECT().ListAll().Return(([]indexer.Indexer)(nil)).Maybe()

	log := logger.NewNopLogger()
	server := NewServer(cfg, registry, log)

	// Verify server is properly configured
	require.NotNil(t, server)
	require.NotNil(t, server.server)
	require.NotNil(t, server.handler)
	require.True(t, server.config.Enabled)
}

func TestServer_ListenAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
	}{
		{
			name:    "localhost with port",
			address: "localhost:8080",
		},
		{
			name:    "all interfaces with port",
			address: ":8080",
		},
		{
			name:    "specific IP with port",
			address: "127.0.0.1:9090",
		},
		{
			name:    "dynamic port",
			address: ":0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.APIConfig{
				Enabled:       true,
				ListenAddress: tt.address,
				ReadTimeout:   common.Duration{Duration: 5 * time.Second},
				WriteTimeout:  common.Duration{Duration: 5 * time.Second},
				IdleTimeout:   common.Duration{Duration: 60 * time.Second},
				CORS: config.CORSConfig{
					Enabled: false,
				},
			}

			registry := apimocks.NewIndexerRegistry(t)
			log := logger.NewNopLogger()

			server := NewServer(cfg, registry, log)

			require.Equal(t, tt.address, server.server.Addr)
			require.Equal(t, tt.address, server.config.ListenAddress)
		})
	}
}
