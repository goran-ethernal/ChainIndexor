package rpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/goran-ethernal/ChainIndexor/pkg/config"
)

// retryableError checks if an error should trigger a retry.
func retryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Connection errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	// Timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "context deadline exceeded") {
		return true
	}

	// Rate limiting
	if strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "rate limit") {
		return true
	}

	// Temporary server errors
	if strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "gateway timeout") {
		return true
	}

	// Connection pool exhausted
	if strings.Contains(errStr, "connection pool") ||
		strings.Contains(errStr, "no available connection") {
		return true
	}

	return false
}

// calculateBackoff computes the backoff duration for a given attempt with jitter.
func calculateBackoff(attempt int, cfg *config.RetryConfig) time.Duration {
	if attempt <= 1 {
		return 0
	}

	// Calculate exponential backoff
	backoff := float64(cfg.InitialBackoff.Duration) * math.Pow(cfg.BackoffMultiplier, float64(attempt-2))

	// Cap at max backoff
	if backoff > float64(cfg.MaxBackoff.Duration) {
		backoff = float64(cfg.MaxBackoff.Duration)
	}

	// Add jitter (±25%)
	jitterRange := backoff * 0.25
	jitter := (rand.Float64() * 2 * jitterRange) - jitterRange
	backoff += jitter

	// Ensure non-negative
	if backoff < 0 {
		backoff = 0
	}

	return time.Duration(backoff)
}

// retryWithBackoff executes a function with exponential backoff retry logic.
// It respects context cancellation and deadlines.
func retryWithBackoff(ctx context.Context, cfg *config.RetryConfig, operation string, fn func() error) error {
	if cfg == nil {
		// No retry config, execute once
		return fn()
	}

	var lastErr error
	startTime := time.Now()

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check context before attempting
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled before attempt %d: %w", attempt, err)
		}

		// Execute the operation
		err := fn()
		if err == nil {
			// Success
			if attempt > 1 {
				// Log retry success metrics
				RPCRetryInc(operation)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !retryableError(err) {
			// Non-retryable error, fail immediately
			return fmt.Errorf("non-retryable error on attempt %d/%d: %w", attempt, cfg.MaxAttempts, err)
		}

		// Check if we have more attempts left
		if attempt >= cfg.MaxAttempts {
			// No more retries
			break
		}

		// Calculate backoff duration
		backoffDuration := calculateBackoff(attempt, cfg)

		// Wait with context awareness
		if backoffDuration > 0 {
			select {
			case <-time.After(backoffDuration):
				// Continue to next attempt
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during backoff (attempt %d/%d): %w",
					attempt, cfg.MaxAttempts, ctx.Err())
			}
		}

		// Increment retry counter
		RPCRetryInc(operation)
	}

	// All retries exhausted
	return fmt.Errorf("all %d attempts failed after %v (last error: %w)",
		cfg.MaxAttempts, time.Since(startTime), lastErr)
}
