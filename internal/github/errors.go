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
