package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goran-ethernal/ChainIndexor/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORSMiddleware(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		require.NoError(t, err)
	})

	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		requestMethod  string
		expectCORS     bool
		expectedOrigin string
	}{
		{
			name:           "wildcard allows any origin",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://example.com",
			requestMethod:  http.MethodGet,
			expectCORS:     true,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "wildcard with no origin header",
			allowedOrigins: []string{"*"},
			requestOrigin:  "",
			requestMethod:  http.MethodGet,
			expectCORS:     true,
			expectedOrigin: "*",
		},
		{
			name:           "specific origin allowed",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://example.com",
			requestMethod:  http.MethodGet,
			expectCORS:     true,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "specific origin not allowed",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://evil.com",
			requestMethod:  http.MethodGet,
			expectCORS:     false,
			expectedOrigin: "",
		},
		{
			name:           "multiple allowed origins - first matches",
			allowedOrigins: []string{"https://example.com", "https://another.com"},
			requestOrigin:  "https://example.com",
			requestMethod:  http.MethodGet,
			expectCORS:     true,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "multiple allowed origins - second matches",
			allowedOrigins: []string{"https://example.com", "https://another.com"},
			requestOrigin:  "https://another.com",
			requestMethod:  http.MethodGet,
			expectCORS:     true,
			expectedOrigin: "https://another.com",
		},
		{
			name:           "multiple allowed origins - none match",
			allowedOrigins: []string{"https://example.com", "https://another.com"},
			requestOrigin:  "https://evil.com",
			requestMethod:  http.MethodGet,
			expectCORS:     false,
			expectedOrigin: "",
		},
		{
			name:           "preflight OPTIONS request with wildcard",
			allowedOrigins: []string{"*"},
			requestOrigin:  "https://example.com",
			requestMethod:  http.MethodOptions,
			expectCORS:     true,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "preflight OPTIONS request with specific origin",
			allowedOrigins: []string{"https://example.com"},
			requestOrigin:  "https://example.com",
			requestMethod:  http.MethodOptions,
			expectCORS:     true,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "empty allowed origins list",
			allowedOrigins: []string{},
			requestOrigin:  "https://example.com",
			requestMethod:  http.MethodGet,
			expectCORS:     false,
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			middleware := CORSMiddleware(tt.allowedOrigins)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest(tt.requestMethod, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if tt.expectCORS {
				require.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
				require.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
				require.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
				require.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
			} else {
				require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}

			// For OPTIONS preflight, expect 200 with no body
			if tt.requestMethod == http.MethodOptions {
				require.Equal(t, http.StatusOK, w.Code)
				require.Empty(t, w.Body.String())
			} else {
				// For regular requests, handler should be called
				require.Equal(t, http.StatusOK, w.Code)
				require.Equal(t, "OK", w.Body.String())
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		path          string
		method        string
		handlerStatus int
		setupHandler  func() http.Handler
	}{
		{
			name:          "logs successful GET request",
			path:          "/api/test",
			method:        http.MethodGet,
			handlerStatus: http.StatusOK,
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			},
		},
		{
			name:          "logs POST request",
			path:          "/api/create",
			method:        http.MethodPost,
			handlerStatus: http.StatusCreated,
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
				})
			},
		},
		{
			name:          "logs error status",
			path:          "/api/error",
			method:        http.MethodGet,
			handlerStatus: http.StatusInternalServerError,
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				})
			},
		},
		{
			name:          "logs 404 not found",
			path:          "/nonexistent",
			method:        http.MethodGet,
			handlerStatus: http.StatusNotFound,
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				})
			},
		},
		{
			name:          "captures status when WriteHeader not explicitly called",
			path:          "/implicit-200",
			method:        http.MethodGet,
			handlerStatus: http.StatusOK,
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Don't call WriteHeader - should default to 200
					_, err := w.Write([]byte("OK"))
					require.NoError(t, err)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := logger.NewNopLogger()
			middleware := LoggingMiddleware(log)

			h := tt.setupHandler()
			wrappedHandler := middleware(h)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			// Verify the handler was called and status was set
			require.Equal(t, tt.handlerStatus, w.Code)
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		writeHeaders   []int
		expectedStatus int
	}{
		{
			name:           "captures status code",
			writeHeaders:   []int{http.StatusCreated},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "captures error status",
			writeHeaders:   []int{http.StatusInternalServerError},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "first WriteHeader wins on underlying writer",
			writeHeaders:   []int{http.StatusOK, http.StatusBadRequest},
			expectedStatus: http.StatusOK, // http.ResponseWriter only honors first WriteHeader
		},
		{
			name:           "defaults to 200 if not set",
			writeHeaders:   []int{},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			for _, status := range tt.writeHeaders {
				wrapped.WriteHeader(status)
			}

			// Check the underlying ResponseWriter (first WriteHeader wins)
			require.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		handler        http.Handler
		expectPanic    bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "handler without panic",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte("success"))
				require.NoError(t, err)
			}),
			expectPanic:    false,
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name: "handler with panic - string",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("something went wrong")
			}),
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name: "handler with panic - error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(assert.AnError)
			}),
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name: "handler with panic - integer",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(42)
			}),
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := logger.NewNopLogger()
			middleware := RecoveryMiddleware(log)
			wrappedHandler := middleware(tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			// Should not panic even if handler panics
			require.NotPanics(t, func() {
				wrappedHandler.ServeHTTP(w, req)
			})

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestMiddlewareChaining(t *testing.T) {
	t.Parallel()

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("final handler"))
		require.NoError(t, err)
	})

	log := logger.NewNopLogger()

	// Chain all middlewares
	handler := RecoveryMiddleware(log)(
		LoggingMiddleware(log)(
			CORSMiddleware([]string{"*"})(
				finalHandler,
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "final handler", w.Body.String())
	require.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSPreflightScenario(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("handler should not be reached for OPTIONS"))
		require.NoError(t, err)
	})

	middleware := CORSMiddleware([]string{"https://example.com", "https://another.com"})
	wrappedHandler := middleware(handler)

	// Preflight request
	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	require.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	require.Empty(t, w.Body.String()) // No body for OPTIONS
}
