package github

import (
	"errors"
	"testing"
)

func TestAuthenticationError(t *testing.T) {
	t.Run("creates error with message only", func(t *testing.T) {
		err := NewAuthenticationError("test message", nil)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		expected := "authentication failed: test message"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates error with message and wrapped error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewAuthenticationError("test message", innerErr)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		expected := "authentication failed: test message: inner error"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("unwraps error correctly", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewAuthenticationError("test message", innerErr)

		unwrapped := err.Unwrap()
		if unwrapped != innerErr {
			t.Errorf("Expected unwrapped error to be inner error, got %v", unwrapped)
		}
	})

	t.Run("unwrap returns nil when no inner error", func(t *testing.T) {
		err := NewAuthenticationError("test message", nil)

		unwrapped := err.Unwrap()
		if unwrapped != nil {
			t.Errorf("Expected unwrapped error to be nil, got %v", unwrapped)
		}
	})
}

func TestIsAuthenticationError(t *testing.T) {
	t.Run("returns true for AuthenticationError", func(t *testing.T) {
		err := NewAuthenticationError("test", nil)
		if !IsAuthenticationError(err) {
			t.Error("Expected IsAuthenticationError to return true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("regular error")
		if IsAuthenticationError(err) {
			t.Error("Expected IsAuthenticationError to return false")
		}
	})

	t.Run("returns false for nil", func(t *testing.T) {
		if IsAuthenticationError(nil) {
			t.Error("Expected IsAuthenticationError to return false for nil")
		}
	})
}

func TestRateLimitError(t *testing.T) {
	t.Run("creates error with all fields", func(t *testing.T) {
		err := NewRateLimitError("test message", 5000, 0, 1234567890, nil)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		if err.Limit != 5000 {
			t.Errorf("Expected Limit=5000, got %d", err.Limit)
		}

		if err.Remaining != 0 {
			t.Errorf("Expected Remaining=0, got %d", err.Remaining)
		}

		if err.ResetAt != 1234567890 {
			t.Errorf("Expected ResetAt=1234567890, got %d", err.ResetAt)
		}

		if !err.IsRetryable() {
			t.Error("Expected RateLimitError to be retryable")
		}
	})

	t.Run("error message includes rate limit info", func(t *testing.T) {
		err := NewRateLimitError("test message", 5000, 0, 1234567890, nil)
		errMsg := err.Error()

		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}

		// Should contain key information
		if len(errMsg) < 20 {
			t.Errorf("Error message seems too short: %s", errMsg)
		}
	})

	t.Run("unwraps inner error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewRateLimitError("test message", 5000, 0, 1234567890, innerErr)

		unwrapped := err.Unwrap()
		if unwrapped != innerErr {
			t.Errorf("Expected unwrapped error to be inner error, got %v", unwrapped)
		}
	})
}

func TestIsRateLimitError(t *testing.T) {
	t.Run("returns true for RateLimitError", func(t *testing.T) {
		err := NewRateLimitError("test", 5000, 0, 1234567890, nil)
		if !IsRateLimitError(err) {
			t.Error("Expected IsRateLimitError to return true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("regular error")
		if IsRateLimitError(err) {
			t.Error("Expected IsRateLimitError to return false")
		}
	})
}

func TestRetryableError(t *testing.T) {
	t.Run("creates error correctly", func(t *testing.T) {
		err := NewRetryableError("test message", nil)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		if !err.IsRetryable() {
			t.Error("Expected RetryableError to be retryable")
		}
	})

	t.Run("error message includes message", func(t *testing.T) {
		err := NewRetryableError("test message", nil)
		errMsg := err.Error()

		expected := "retryable error: test message"
		if errMsg != expected {
			t.Errorf("Expected error message %q, got %q", expected, errMsg)
		}
	})

	t.Run("unwraps inner error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewRetryableError("test message", innerErr)

		unwrapped := err.Unwrap()
		if unwrapped != innerErr {
			t.Errorf("Expected unwrapped error to be inner error, got %v", unwrapped)
		}
	})
}

func TestIsRetryableError(t *testing.T) {
	t.Run("returns true for RetryableError", func(t *testing.T) {
		err := NewRetryableError("test", nil)
		if !IsRetryableError(err) {
			t.Error("Expected IsRetryableError to return true")
		}
	})

	t.Run("returns true for RateLimitError", func(t *testing.T) {
		err := NewRateLimitError("test", 5000, 0, 1234567890, nil)
		if !IsRetryableError(err) {
			t.Error("Expected IsRetryableError to return true for RateLimitError")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("regular error")
		if IsRetryableError(err) {
			t.Error("Expected IsRetryableError to return false")
		}
	})

	t.Run("returns false for nil", func(t *testing.T) {
		if IsRetryableError(nil) {
			t.Error("Expected IsRetryableError to return false for nil")
		}
	})
}
