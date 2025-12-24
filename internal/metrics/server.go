package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is the HTTP server that exposes Prometheus metrics.
type Server struct {
	config *config.MetricsConfig
	server *http.Server
	stopCh chan struct{}
}

// NewServer creates a new metrics server.
func NewServer(config *config.MetricsConfig) *Server {
	return &Server{
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Start starts the metrics HTTP server and begins collecting system metrics.
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	mux := http.NewServeMux()

	// Register Prometheus metrics handler
	mux.Handle(s.config.Path, promhttp.Handler())

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	s.server = &http.Server{
		Addr:              s.config.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start system metrics updater
	go s.updateSystemMetrics(ctx)

	// Start the server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error (in a real implementation, use proper logging)
			fmt.Printf("metrics server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the metrics HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	close(s.stopCh)

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown metrics server: %w", err)
	}

	return nil
}

// updateSystemMetrics periodically updates system-level metrics.
func (s *Server) updateSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			UpdateSystemMetrics()
		case <-ctx.Done():
			// Context cancelled, before stop
			return
		case <-s.stopCh:
			// stop called before context cancelled
			return
		}
	}
}
