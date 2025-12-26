package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/goran-ethernal/ChainIndexor/internal/common"
	"github.com/goran-ethernal/ChainIndexor/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNetError implements net.Error for testing
type mockNetError struct {
	msg       string
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return e.msg }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

func TestRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "network timeout error",
			err:       &mockNetError{msg: "network timeout", timeout: true},
			retryable: true,
		},
		{
			name:      "connection refused",
			err:       syscall.ECONNREFUSED,
			retryable: true,
		},
		{
			name:      "connection reset",
			err:       syscall.ECONNRESET,
			retryable: true,
		},
		{
			name:      "broken pipe",
			err:       syscall.EPIPE,
			retryable: true,
		},
		{
			name:      "timeout string",
			err:       errors.New("operation timeout"),
			retryable: true,
		},
		{
			name:      "deadline exceeded",
			err:       errors.New("deadline exceeded"),
			retryable: true,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			retryable: true,
		},
		{
			name:      "rate limit 429",
			err:       errors.New("HTTP 429"),
			retryable: true,
		},
		{
			name:      "too many requests",
			err:       errors.New("too many requests"),
			retryable: true,
		},
		{
			name:      "rate limit",
			err:       errors.New("rate limit exceeded"),
			retryable: true,
		},
		{
			name:      "502 bad gateway",
			err:       errors.New("502 bad gateway"),
			retryable: true,
		},
		{
			name:      "503 service unavailable",
			err:       errors.New("503 Service Unavailable"),
			retryable: true,
		},
		{
			name:      "504 gateway timeout",
			err:       errors.New("504 Gateway Timeout"),
			retryable: true,
		},
		{
			name:      "connection pool exhausted",
			err:       errors.New("connection pool exhausted"),
			retryable: true,
		},
		{
			name:      "no available connection",
			err:       errors.New("no available connection"),
			retryable: true,
		},
		{
			name:      "invalid parameter",
			err:       errors.New("invalid parameter"),
			retryable: false,
		},
		{
			name:      "authentication failed",
			err:       errors.New("401 Unauthorized"),
			retryable: false,
		},
		{
			name:      "not found",
			err:       errors.New("404 Not Found"),
			retryable: false,
		},
		{
			name:      "bad request",
			err:       errors.New("400 Bad Request"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := retryableError(tt.err)
			assert.Equal(t, tt.retryable, result, "retryableError(%v) = %v, want %v", tt.err, result, tt.retryable)
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := &config.RetryConfig{
		InitialBackoff:    common.NewDuration(1 * time.Second),
		MaxBackoff:        common.NewDuration(30 * time.Second),
		BackoffMultiplier: 2.0,
	}

	tests := []struct {
		name        string
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{
			name:        "attempt 1 - no backoff",
			attempt:     1,
			minExpected: 0,
			maxExpected: 0,
		},
		{
			name:        "attempt 2 - initial backoff with jitter",
			attempt:     2,
			minExpected: 750 * time.Millisecond,  // 1s - 25%
			maxExpected: 1250 * time.Millisecond, // 1s + 25%
		},
		{
			name:        "attempt 3 - exponential backoff",
			attempt:     3,
			minExpected: 1500 * time.Millisecond, // 2s - 25%
			maxExpected: 2500 * time.Millisecond, // 2s + 25%
		},
		{
			name:        "attempt 4",
			attempt:     4,
			minExpected: 3 * time.Second, // 4s - 25%
			maxExpected: 5 * time.Second, // 4s + 25%
		},
		{
			name:        "attempt 5",
			attempt:     5,
			minExpected: 6 * time.Second,  // 8s - 25%
			maxExpected: 10 * time.Second, // 8s + 25%
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for jitter randomness
			for i := 0; i < 10; i++ {
				backoff := calculateBackoff(tt.attempt, cfg)
				assert.GreaterOrEqual(t, backoff, tt.minExpected, "backoff should be >= min")
				assert.LessOrEqual(t, backoff, tt.maxExpected, "backoff should be <= max")
			}
		})
	}
}

func TestCalculateBackoff_CappedAtMax(t *testing.T) {
	cfg := &config.RetryConfig{
		InitialBackoff:    common.NewDuration(1 * time.Second),
		MaxBackoff:        common.NewDuration(5 * time.Second),
		BackoffMultiplier: 2.0,
	}

	// Attempt 6 would be 32s without cap, should be capped at 5s (plus jitter)
	backoff := calculateBackoff(10, cfg)
	assert.LessOrEqual(t, backoff, 6250*time.Millisecond, "backoff should be capped at max + 25% jitter")
}

func TestRetryWithBackoff_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    common.NewDuration(10 * time.Millisecond),
		MaxBackoff:        common.NewDuration(100 * time.Millisecond),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "should succeed on first attempt")
}

func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	cfg := &config.RetryConfig{
		MaxAttempts:       5,
		InitialBackoff:    common.NewDuration(10 * time.Millisecond),
		MaxBackoff:        common.NewDuration(100 * time.Millisecond),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return &mockNetError{msg: "temporary error", timeout: true}
		}
		return nil
	}

	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	require.NoError(t, err)
	assert.Equal(t, 3, callCount, "should succeed on third attempt")
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	cfg := &config.RetryConfig{
		MaxAttempts:       5,
		InitialBackoff:    common.NewDuration(10 * time.Millisecond),
		MaxBackoff:        common.NewDuration(100 * time.Millisecond),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	expectedErr := errors.New("invalid parameter")
	fn := func() error {
		callCount++
		return expectedErr
	}

	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-retryable error")
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 1, callCount, "should not retry non-retryable error")
}

func TestRetryWithBackoff_ExhaustedRetries(t *testing.T) {
	ctx := context.Background()
	cfg := &config.RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    common.NewDuration(10 * time.Millisecond),
		MaxBackoff:        common.NewDuration(100 * time.Millisecond),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	expectedErr := &mockNetError{msg: "persistent error", timeout: true}
	fn := func() error {
		callCount++
		return expectedErr
	}

	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 attempts failed")
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 3, callCount, "should retry max attempts")
}

func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := &config.RetryConfig{
		MaxAttempts:       5,
		InitialBackoff:    common.NewDuration(10 * time.Millisecond),
		MaxBackoff:        common.NewDuration(100 * time.Millisecond),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after second attempt
		}
		return &mockNetError{msg: "temporary error", timeout: true}
	}

	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
	assert.Equal(t, 2, callCount, "should stop retrying after context cancelled")
}

func TestRetryWithBackoff_ContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := &config.RetryConfig{
		MaxAttempts:       10,
		InitialBackoff:    common.NewDuration(100 * time.Millisecond),
		MaxBackoff:        common.NewDuration(1 * time.Second),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return &mockNetError{msg: "temporary error", timeout: true}
	}

	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
	// Should fail early due to context deadline, not reach max attempts
	assert.Less(t, callCount, 10, "should stop before max attempts due to deadline")
}

func TestRetryWithBackoff_NilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := retryWithBackoff(ctx, nil, "test_operation", fn)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount, "should execute once without retry config")
}

func TestRetryWithBackoff_NilConfigWithError(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	expectedErr := errors.New("some error")
	fn := func() error {
		callCount++
		return expectedErr
	}

	err := retryWithBackoff(ctx, nil, "test_operation", fn)
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 1, callCount, "should execute once without retry config")
}

func TestRetryWithBackoff_BackoffTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing test in short mode")
	}

	ctx := context.Background()
	cfg := &config.RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    common.NewDuration(100 * time.Millisecond),
		MaxBackoff:        common.NewDuration(500 * time.Millisecond),
		BackoffMultiplier: 2.0,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return &mockNetError{msg: "temporary error", timeout: true}
	}

	start := time.Now()
	err := retryWithBackoff(ctx, cfg, "test_operation", fn)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, 3, callCount, "should make 3 attempts")
	// With jitter (±25%), minimum time should be:
	// attempt 1: no wait
	// attempt 2: 75ms (100ms - 25%)
	// attempt 3: 150ms (200ms - 25%)
	// Total minimum: ~225ms, but allowing for some timing variance
	assert.Greater(t, elapsed, 50*time.Millisecond, "should respect backoff timing")
}

func TestRetryableError_WrappedErrors(t *testing.T) {
	// Test that wrapped errors are properly detected
	baseErr := syscall.ECONNREFUSED
	wrappedErr := fmt.Errorf("connection failed: %w", baseErr)

	result := retryableError(wrappedErr)
	assert.True(t, result, "should detect wrapped connection refused error")
}

func TestRetryableError_NetworkError(t *testing.T) {
	// Test real net.OpError
	netErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: syscall.ECONNREFUSED,
	}

	result := retryableError(netErr)
	assert.True(t, result, "should detect net.OpError as retryable")
}
