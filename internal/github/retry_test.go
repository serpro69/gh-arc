package github

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxRetries != DefaultMaxRetries {
		t.Errorf("Expected MaxRetries=%d, got %d", DefaultMaxRetries, policy.MaxRetries)
	}

	if policy.BaseDelay != DefaultBaseDelay {
		t.Errorf("Expected BaseDelay=%v, got %v", DefaultBaseDelay, policy.BaseDelay)
	}

	if policy.MaxDelay != DefaultMaxDelay {
		t.Errorf("Expected MaxDelay=%v, got %v", DefaultMaxDelay, policy.MaxDelay)
	}
}

func TestCalculateBackoff(t *testing.T) {
	policy := &RetryPolicy{
		BaseDelay: 1 * time.Second,
		MaxDelay:  10 * time.Second,
	}

	tests := []struct {
		name        string
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration
	}{
		{"first attempt", 0, 1 * time.Second, 2 * time.Second},
		{"second attempt", 1, 2 * time.Second, 4 * time.Second},
		{"third attempt", 2, 4 * time.Second, 8 * time.Second},
		{"capped attempt", 10, 10 * time.Second, 15 * time.Second}, // Should be capped at MaxDelay + jitter
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := policy.calculateBackoff(tt.attempt)

			if backoff < tt.minExpected {
				t.Errorf("Backoff %v is less than minimum expected %v", backoff, tt.minExpected)
			}

			if backoff > tt.maxExpected {
				t.Errorf("Backoff %v is greater than maximum expected %v", backoff, tt.maxExpected)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{
			name:     "success",
			resp:     &http.Response{StatusCode: 200},
			err:      nil,
			expected: false,
		},
		{
			name:     "rate limited",
			resp:     &http.Response{StatusCode: 429},
			err:      nil,
			expected: true,
		},
		{
			name:     "server error 500",
			resp:     &http.Response{StatusCode: 500},
			err:      nil,
			expected: true,
		},
		{
			name:     "server error 503",
			resp:     &http.Response{StatusCode: 503},
			err:      nil,
			expected: true,
		},
		{
			name:     "client error 404",
			resp:     &http.Response{StatusCode: 404},
			err:      nil,
			expected: false,
		},
		{
			name:     "retryable error",
			resp:     nil,
			err:      NewRetryableError("test", nil),
			expected: true,
		},
		{
			name:     "non-retryable error",
			resp:     nil,
			err:      errors.New("test error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRetry(tt.resp, tt.err)
			if result != tt.expected {
				t.Errorf("ShouldRetry() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second)

		if cb.State() != CircuitClosed {
			t.Errorf("Expected CircuitClosed, got %v", cb.State())
		}

		if !cb.Allow() {
			t.Error("Expected Allow() to return true for closed circuit")
		}
	})

	t.Run("opens after max failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 1*time.Second)

		// Record 3 failures
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordFailure()

		if cb.State() != CircuitOpen {
			t.Errorf("Expected CircuitOpen, got %v", cb.State())
		}

		if cb.Allow() {
			t.Error("Expected Allow() to return false for open circuit")
		}
	})

	t.Run("transitions to half-open after reset timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond)

		// Open the circuit
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordFailure()

		if cb.State() != CircuitOpen {
			t.Errorf("Expected CircuitOpen, got %v", cb.State())
		}

		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)

		// Should transition to half-open
		if !cb.Allow() {
			t.Error("Expected Allow() to return true after reset timeout")
		}

		if cb.State() != CircuitHalfOpen {
			t.Errorf("Expected CircuitHalfOpen, got %v", cb.State())
		}
	})

	t.Run("closes after success in half-open", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond)

		// Open the circuit
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordFailure()

		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)

		// Transition to half-open
		cb.Allow()

		// Record success
		cb.RecordSuccess()

		if cb.State() != CircuitClosed {
			t.Errorf("Expected CircuitClosed after success, got %v", cb.State())
		}

		if !cb.Allow() {
			t.Error("Expected Allow() to return true for closed circuit")
		}
	})
}

func TestRetryWithBackoff(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		}

		attempts := 0
		fn := func() (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: 200}, nil
		}

		ctx := context.Background()
		resp, err := RetryWithBackoff(ctx, policy, fn)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if resp == nil || resp.StatusCode != 200 {
			t.Error("Expected successful response")
		}

		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retries on 500 error", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxRetries: 2,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		}

		attempts := 0
		fn := func() (*http.Response, error) {
			attempts++
			if attempts < 3 {
				return &http.Response{StatusCode: 500}, nil
			}
			return &http.Response{StatusCode: 200}, nil
		}

		ctx := context.Background()
		resp, err := RetryWithBackoff(ctx, policy, fn)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if resp == nil || resp.StatusCode != 200 {
			t.Error("Expected successful response after retries")
		}

		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("exhausts retries", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxRetries: 2,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
		}

		attempts := 0
		fn := func() (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: 500}, nil
		}

		ctx := context.Background()
		_, err := RetryWithBackoff(ctx, policy, fn)

		if err == nil {
			t.Error("Expected error after exhausting retries")
		}

		// MaxRetries=2 means 3 total attempts (initial + 2 retries)
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxRetries: 10,
			BaseDelay:  100 * time.Millisecond,
			MaxDelay:   1 * time.Second,
		}

		attempts := 0
		fn := func() (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: 500}, nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		// Cancel after first retry
		go func() {
			time.Sleep(150 * time.Millisecond)
			cancel()
		}()

		_, err := RetryWithBackoff(ctx, policy, fn)

		if err == nil {
			t.Error("Expected error due to context cancellation")
		}

		// Should only attempt once or twice before cancellation
		if attempts > 3 {
			t.Errorf("Expected at most 3 attempts before cancellation, got %d", attempts)
		}
	})
}
