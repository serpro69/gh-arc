package github

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryFunc is a function that can be retried
type RetryFunc func() (*http.Response, error)

// RetryPolicy defines the retry behavior
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryPolicy returns a retry policy with sensible defaults
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: DefaultMaxRetries,
		BaseDelay:  DefaultBaseDelay,
		MaxDelay:   DefaultMaxDelay,
	}
}

// calculateBackoff calculates the backoff duration with exponential backoff and jitter
func (p *RetryPolicy) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential backoff: baseDelay * 2^attempt
	backoff := float64(p.BaseDelay) * math.Pow(2, float64(attempt))

	// Cap at max delay
	if backoff > float64(p.MaxDelay) {
		backoff = float64(p.MaxDelay)
	}

	// Add jitter (random value between 0 and backoff/2)
	// This prevents thundering herd problem
	jitter := rand.Float64() * (backoff / 2)
	backoff = backoff + jitter

	return time.Duration(backoff)
}

// ShouldRetry determines if an HTTP response should be retried
func ShouldRetry(resp *http.Response, err error) bool {
	// If there's an error, check if it's retryable
	if err != nil {
		return IsRetryableError(err)
	}

	// If no error, check HTTP status code
	if resp == nil {
		return false
	}

	// Retry on rate limit (429)
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	// Retry on server errors (5xx)
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		return true
	}

	// Don't retry on other status codes
	return false
}

// RetryWithBackoff retries a function with exponential backoff
// It returns the last response and error encountered
func RetryWithBackoff(ctx context.Context, policy *RetryPolicy, fn RetryFunc) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		// Execute the function
		resp, err := fn()

		// Check if we should retry
		if !ShouldRetry(resp, err) {
			// Success or non-retryable error
			return resp, err
		}

		// Save the last response and error
		lastResp = resp
		lastErr = err

		// If this was the last attempt, don't wait
		if attempt == policy.MaxRetries {
			break
		}

		// Calculate backoff delay
		backoff := policy.calculateBackoff(attempt)

		// Check for rate limit and adjust delay if needed
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			rateLimitInfo := ParseRateLimitHeaders(resp.Header)
			if rateLimitInfo != nil && !rateLimitInfo.Reset.IsZero() {
				// Wait until rate limit resets (plus a small buffer)
				resetDelay := rateLimitInfo.TimeUntilReset() + time.Second
				if resetDelay > backoff {
					backoff = resetDelay
				}
			}
		}

		// Wait before retrying, respecting context cancellation
		select {
		case <-ctx.Done():
			return lastResp, fmt.Errorf("retry canceled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	// All retries exhausted
	if lastErr != nil {
		return lastResp, fmt.Errorf("max retries exceeded: %w", lastErr)
	}

	// If we have a response but no error, wrap it in a retryable error
	if lastResp != nil {
		return lastResp, NewRetryableError(
			fmt.Sprintf("max retries exceeded (status: %d)", lastResp.StatusCode),
			nil,
		)
	}

	return nil, fmt.Errorf("max retries exceeded with no response")
}

// CircuitBreaker tracks failures and can temporarily stop requests
type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	failureCount    int
	lastFailureTime time.Time
	state           CircuitState
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// CircuitClosed means requests are allowed
	CircuitClosed CircuitState = iota
	// CircuitOpen means requests are blocked
	CircuitOpen
	// CircuitHalfOpen means testing if service recovered
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
	}
}

// Allow checks if a request should be allowed through
func (cb *CircuitBreaker) Allow() bool {
	now := time.Now()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		// Check if enough time has passed to try again
		if now.Sub(cb.lastFailureTime) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false

	case CircuitHalfOpen:
		return true
	}

	return false
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.failureCount = 0
	cb.state = CircuitClosed
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	return cb.state
}
