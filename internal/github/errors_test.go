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

func TestAuthorizationError(t *testing.T) {
	t.Run("creates error with message only", func(t *testing.T) {
		err := NewAuthorizationError("insufficient permissions", "", nil)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		expected := "authorization failed: insufficient permissions"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates error with resource", func(t *testing.T) {
		err := NewAuthorizationError("insufficient permissions", "repo:write", nil)

		expected := "authorization failed: insufficient permissions (resource: repo:write)"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates error with wrapped error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewAuthorizationError("insufficient permissions", "repo:write", innerErr)

		expected := "authorization failed: insufficient permissions (resource: repo:write): inner error"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("unwraps error correctly", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewAuthorizationError("test", "", innerErr)

		unwrapped := err.Unwrap()
		if unwrapped != innerErr {
			t.Errorf("Expected unwrapped error to be inner error, got %v", unwrapped)
		}
	})
}

func TestIsAuthorizationError(t *testing.T) {
	t.Run("returns true for AuthorizationError", func(t *testing.T) {
		err := NewAuthorizationError("test", "", nil)
		if !IsAuthorizationError(err) {
			t.Error("Expected IsAuthorizationError to return true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("regular error")
		if IsAuthorizationError(err) {
			t.Error("Expected IsAuthorizationError to return false")
		}
	})
}

func TestNotFoundError(t *testing.T) {
	t.Run("creates error with message only", func(t *testing.T) {
		err := NewNotFoundError("resource not found", "", nil)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		expected := "not found: resource not found"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates error with resource", func(t *testing.T) {
		err := NewNotFoundError("repository not found", "owner/repo", nil)

		expected := "not found: repository not found (resource: owner/repo)"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates error with wrapped error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewNotFoundError("repository not found", "owner/repo", innerErr)

		expected := "not found: repository not found (resource: owner/repo): inner error"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("unwraps error correctly", func(t *testing.T) {
		innerErr := errors.New("inner error")
		err := NewNotFoundError("test", "", innerErr)

		unwrapped := err.Unwrap()
		if unwrapped != innerErr {
			t.Errorf("Expected unwrapped error to be inner error, got %v", unwrapped)
		}
	})
}

func TestIsNotFoundError(t *testing.T) {
	t.Run("returns true for NotFoundError", func(t *testing.T) {
		err := NewNotFoundError("test", "", nil)
		if !IsNotFoundError(err) {
			t.Error("Expected IsNotFoundError to return true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("regular error")
		if IsNotFoundError(err) {
			t.Error("Expected IsNotFoundError to return false")
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("creates error with message only", func(t *testing.T) {
		err := NewValidationError("invalid input", nil, nil)

		if err == nil {
			t.Fatal("Expected error to be created, got nil")
		}

		expected := "validation failed: invalid input"
		if err.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates error with field errors", func(t *testing.T) {
		fieldErrors := []FieldError{
			{Field: "name", Code: "missing", Message: "name is required"},
			{Field: "email", Code: "invalid", Message: "email is invalid"},
		}
		err := NewValidationError("validation failed", fieldErrors, nil)

		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}

		// Check that field errors are included
		if len(err.Errors) != 2 {
			t.Errorf("Expected 2 field errors, got %d", len(err.Errors))
		}
	})

	t.Run("creates error with wrapped error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		fieldErrors := []FieldError{
			{Field: "name", Code: "missing", Message: "name is required"},
		}
		err := NewValidationError("validation failed", fieldErrors, innerErr)

		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}

		// Check unwrap works
		if err.Unwrap() != innerErr {
			t.Error("Expected to unwrap inner error")
		}
	})
}

func TestIsValidationError(t *testing.T) {
	t.Run("returns true for ValidationError", func(t *testing.T) {
		err := NewValidationError("test", nil, nil)
		if !IsValidationError(err) {
			t.Error("Expected IsValidationError to return true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		err := errors.New("regular error")
		if IsValidationError(err) {
			t.Error("Expected IsValidationError to return false")
		}
	})
}

func TestErrorResponse(t *testing.T) {
	t.Run("error message with no field errors", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Bad Request",
			StatusCode: 400,
		}

		expected := "GitHub API error (status 400): Bad Request"
		if errResp.Error() != expected {
			t.Errorf("Expected error message %q, got %q", expected, errResp.Error())
		}
	})

	t.Run("error message with field errors", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Validation Failed",
			StatusCode: 422,
			Errors: []FieldError{
				{Field: "name", Code: "missing", Message: "name is required"},
			},
		}

		errMsg := errResp.Error()
		if errMsg == "" {
			t.Error("Expected non-empty error message")
		}

		// Should include status, message, and field error
		if len(errMsg) < 20 {
			t.Errorf("Error message seems too short: %s", errMsg)
		}
	})
}

func TestParseErrorResponse(t *testing.T) {
	t.Run("returns error for nil response", func(t *testing.T) {
		err := ParseErrorResponse(nil)
		if err == nil {
			t.Error("Expected error for nil response")
		}
	})

	t.Run("parses 401 as AuthenticationError", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Bad credentials",
			StatusCode: 401,
		}

		err := ParseErrorResponse(errResp)
		if !IsAuthenticationError(err) {
			t.Errorf("Expected AuthenticationError, got %T", err)
		}
	})

	t.Run("parses 403 as AuthorizationError", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Forbidden",
			StatusCode: 403,
		}

		err := ParseErrorResponse(errResp)
		if !IsAuthorizationError(err) {
			t.Errorf("Expected AuthorizationError, got %T", err)
		}
	})

	t.Run("parses 404 as NotFoundError", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Not Found",
			StatusCode: 404,
		}

		err := ParseErrorResponse(errResp)
		if !IsNotFoundError(err) {
			t.Errorf("Expected NotFoundError, got %T", err)
		}
	})

	t.Run("parses 422 as ValidationError", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Validation Failed",
			StatusCode: 422,
			Errors: []FieldError{
				{Field: "name", Code: "missing", Message: "name is required"},
			},
		}

		err := ParseErrorResponse(errResp)
		if !IsValidationError(err) {
			t.Errorf("Expected ValidationError, got %T", err)
		}

		// Verify field errors are preserved
		valErr := err.(*ValidationError)
		if len(valErr.Errors) != 1 {
			t.Errorf("Expected 1 field error, got %d", len(valErr.Errors))
		}
	})

	t.Run("parses 429 as RateLimitError", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "API rate limit exceeded",
			StatusCode: 429,
		}

		err := ParseErrorResponse(errResp)
		if !IsRateLimitError(err) {
			t.Errorf("Expected RateLimitError, got %T", err)
		}
	})

	t.Run("returns ErrorResponse for unknown status codes", func(t *testing.T) {
		errResp := &ErrorResponse{
			Message:    "Server Error",
			StatusCode: 500,
		}

		err := ParseErrorResponse(errResp)
		if _, ok := err.(*ErrorResponse); !ok {
			t.Errorf("Expected ErrorResponse, got %T", err)
		}
	})
}
