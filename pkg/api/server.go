package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/goran-ethernal/ChainIndexor/pkg/api/docs"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/goran-ethernal/ChainIndexor/pkg/rpc"
)

// Ensure docs are initialized
var _ = docs.SwaggerInfo

const shutdownCtxTimeout = 10 * time.Second

// Server represents the API HTTP server.
type Server struct {
	config   *config.APIConfig
	registry IndexerRegistry
	handler  *Handler
	server   *http.Server
	log      *logger.Logger
	rpc      rpc.EthClient
}

// NewServer creates a new API server.
func NewServer(cfg *config.APIConfig, registry IndexerRegistry, rpcClient rpc.EthClient, log *logger.Logger) *Server {
	handler := NewHandler(registry, rpcClient, log)

	mux := http.NewServeMux()

	// Health and info endpoints
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /api/v1/indexers", handler.ListIndexers)

	// Event query endpoints - use indexer name for unique identification
	mux.HandleFunc("GET /api/v1/indexers/{name}/events", handler.GetEvents)
	mux.HandleFunc("GET /api/v1/indexers/{name}/stats", handler.GetStats)

	// Analytics endpoints
	mux.HandleFunc("GET /api/v1/indexers/{name}/events/timeseries", handler.GetEventsTimeseries)
	mux.HandleFunc("GET /api/v1/indexers/{name}/metrics", handler.GetMetrics)

	// Swagger documentation endpoints
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
	))

	// Apply middleware
	var h http.Handler = mux
	h = RecoveryMiddleware(log)(h)
	h = LoggingMiddleware(log)(h)

	if cfg.CORS.Enabled {
		h = CORSMiddleware(cfg.CORS.AllowedOrigins)(h)
	}

	// Use configured timeouts (defaults already applied in config.ApplyDefaults)
	httpServer := &http.Server{
		Addr:         cfg.ListenAddress,
		Handler:      h,
		ReadTimeout:  cfg.ReadTimeout.Duration,
		WriteTimeout: cfg.WriteTimeout.Duration,
		IdleTimeout:  cfg.IdleTimeout.Duration,
	}

	return &Server{
		config:   cfg,
		registry: registry,
		handler:  handler,
		server:   httpServer,
		log:      log,
		rpc:      rpcClient,
	}
}

// Start starts the API server.
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.log.Info("API server is disabled")
		return nil
	}

	s.log.Infof("Starting API server on %s", s.config.ListenAddress)

	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Errorf("API server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownCtxTimeout)
	defer cancel()

	s.log.Info("Shutting down API server...")
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("API server shutdown error: %w", err)
	}

	s.log.Info("API server stopped")
	return nil
}
