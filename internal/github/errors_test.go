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
