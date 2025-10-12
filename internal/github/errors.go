package github

import "fmt"

// AuthenticationError represents an authentication failure (401)
type AuthenticationError struct {
	Message string
	Err     error
}

func (e *AuthenticationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("authentication failed: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("authentication failed: %s", e.Message)
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

// NewAuthenticationError creates a new AuthenticationError
func NewAuthenticationError(message string, err error) *AuthenticationError {
	return &AuthenticationError{
		Message: message,
		Err:     err,
	}
}

// IsAuthenticationError checks if an error is an AuthenticationError
func IsAuthenticationError(err error) bool {
	_, ok := err.(*AuthenticationError)
	return ok
}

// RateLimitError represents a rate limit error (429)
type RateLimitError struct {
	Message   string
	Limit     int
	Remaining int
	ResetAt   int64 // Unix timestamp
	Err       error
}

func (e *RateLimitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("rate limit exceeded: %s (limit: %d, remaining: %d, resets at: %d): %v",
			e.Message, e.Limit, e.Remaining, e.ResetAt, e.Err)
	}
	return fmt.Sprintf("rate limit exceeded: %s (limit: %d, remaining: %d, resets at: %d)",
		e.Message, e.Limit, e.Remaining, e.ResetAt)
}

func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true since rate limit errors are transient
func (e *RateLimitError) IsRetryable() bool {
	return true
}

// NewRateLimitError creates a new RateLimitError
func NewRateLimitError(message string, limit, remaining int, resetAt int64, err error) *RateLimitError {
	return &RateLimitError{
		Message:   message,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
		Err:       err,
	}
}

// IsRateLimitError checks if an error is a RateLimitError
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// RetryableError represents an error that can be retried
type RetryableError struct {
	Message string
	Err     error
}

func (e *RetryableError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("retryable error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("retryable error: %s", e.Message)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true
func (e *RetryableError) IsRetryable() bool {
	return true
}

// NewRetryableError creates a new RetryableError
func NewRetryableError(message string, err error) *RetryableError {
	return &RetryableError{
		Message: message,
		Err:     err,
	}
}

// IsRetryableError checks if an error is retryable
// This checks for both explicit RetryableError and RateLimitError types
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's explicitly a RetryableError
	if _, ok := err.(*RetryableError); ok {
		return true
	}

	// Check if it's a RateLimitError
	if _, ok := err.(*RateLimitError); ok {
		return true
	}

	// Check if the error implements IsRetryable method
	type retryable interface {
		IsRetryable() bool
	}

	if r, ok := err.(retryable); ok {
		return r.IsRetryable()
	}

	return false
}
