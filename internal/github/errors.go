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

// AuthorizationError represents an authorization failure (403)
type AuthorizationError struct {
	Message  string
	Resource string // The resource that was denied
	Err      error
}

func (e *AuthorizationError) Error() string {
	if e.Resource != "" {
		if e.Err != nil {
			return fmt.Sprintf("authorization failed: %s (resource: %s): %v", e.Message, e.Resource, e.Err)
		}
		return fmt.Sprintf("authorization failed: %s (resource: %s)", e.Message, e.Resource)
	}
	if e.Err != nil {
		return fmt.Sprintf("authorization failed: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("authorization failed: %s", e.Message)
}

func (e *AuthorizationError) Unwrap() error {
	return e.Err
}

// NewAuthorizationError creates a new AuthorizationError
func NewAuthorizationError(message string, resource string, err error) *AuthorizationError {
	return &AuthorizationError{
		Message:  message,
		Resource: resource,
		Err:      err,
	}
}

// IsAuthorizationError checks if an error is an AuthorizationError
func IsAuthorizationError(err error) bool {
	_, ok := err.(*AuthorizationError)
	return ok
}

// NotFoundError represents a resource not found error (404)
type NotFoundError struct {
	Message  string
	Resource string // The resource that was not found
	Err      error
}

func (e *NotFoundError) Error() string {
	if e.Resource != "" {
		if e.Err != nil {
			return fmt.Sprintf("not found: %s (resource: %s): %v", e.Message, e.Resource, e.Err)
		}
		return fmt.Sprintf("not found: %s (resource: %s)", e.Message, e.Resource)
	}
	if e.Err != nil {
		return fmt.Sprintf("not found: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("not found: %s", e.Message)
}

func (e *NotFoundError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(message string, resource string, err error) *NotFoundError {
	return &NotFoundError{
		Message:  message,
		Resource: resource,
		Err:      err,
	}
}

// IsNotFoundError checks if an error is a NotFoundError
func IsNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// ValidationError represents a validation error (422)
type ValidationError struct {
	Message string
	Errors  []FieldError // Field-specific validation errors
	Err     error
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string // The field that failed validation
	Code    string // Error code (e.g., "missing", "invalid", "already_exists")
	Message string // Human-readable error message
}

func (e *ValidationError) Error() string {
	if len(e.Errors) > 0 {
		fieldErrs := ""
		for i, fe := range e.Errors {
			if i > 0 {
				fieldErrs += ", "
			}
			fieldErrs += fmt.Sprintf("%s: %s", fe.Field, fe.Message)
		}
		if e.Err != nil {
			return fmt.Sprintf("validation failed: %s [%s]: %v", e.Message, fieldErrs, e.Err)
		}
		return fmt.Sprintf("validation failed: %s [%s]", e.Message, fieldErrs)
	}
	if e.Err != nil {
		return fmt.Sprintf("validation failed: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new ValidationError
func NewValidationError(message string, errors []FieldError, err error) *ValidationError {
	return &ValidationError{
		Message: message,
		Errors:  errors,
		Err:     err,
	}
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// ErrorResponse represents GitHub's error response format
// This matches the structure of GitHub API error responses
type ErrorResponse struct {
	Message          string       `json:"message"`
	DocumentationURL string       `json:"documentation_url,omitempty"`
	Errors           []FieldError `json:"errors,omitempty"`
	StatusCode       int          `json:"-"` // HTTP status code (not in JSON)
}

func (e *ErrorResponse) Error() string {
	if len(e.Errors) > 0 {
		fieldErrs := ""
		for i, fe := range e.Errors {
			if i > 0 {
				fieldErrs += ", "
			}
			fieldErrs += fmt.Sprintf("%s: %s", fe.Field, fe.Message)
		}
		return fmt.Sprintf("GitHub API error (status %d): %s [%s]", e.StatusCode, e.Message, fieldErrs)
	}
	return fmt.Sprintf("GitHub API error (status %d): %s", e.StatusCode, e.Message)
}

// ParseErrorResponse converts a GitHub API ErrorResponse to an appropriate error type
func ParseErrorResponse(errResp *ErrorResponse) error {
	if errResp == nil {
		return fmt.Errorf("unknown error")
	}

	switch errResp.StatusCode {
	case 401:
		return NewAuthenticationError(errResp.Message, nil)
	case 403:
		return NewAuthorizationError(errResp.Message, "", nil)
	case 404:
		return NewNotFoundError(errResp.Message, "", nil)
	case 422:
		return NewValidationError(errResp.Message, errResp.Errors, nil)
	case 429:
		// Rate limit errors should be created with proper rate limit info
		// This is a fallback for when rate limit headers aren't available
		return NewRateLimitError(errResp.Message, 0, 0, 0, nil)
	default:
		// For other status codes, return the ErrorResponse itself
		return errResp
	}
}
