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

// CircularDependencyError represents a circular dependency in stacked PRs
type CircularDependencyError struct {
	Message      string
	Branches     []string // The branches forming the circular dependency
	CurrentPR    int      // Current PR number
	ConflictingPR int     // PR number that creates the circular dependency
	Err          error
}

func (e *CircularDependencyError) Error() string {
	if len(e.Branches) > 0 {
		branchChain := ""
		for i, branch := range e.Branches {
			if i > 0 {
				branchChain += " → "
			}
			branchChain += branch
		}
		if e.Err != nil {
			return fmt.Sprintf("circular dependency detected: %s (dependency chain: %s): %v", e.Message, branchChain, e.Err)
		}
		return fmt.Sprintf("circular dependency detected: %s (dependency chain: %s)", e.Message, branchChain)
	}
	if e.Err != nil {
		return fmt.Sprintf("circular dependency detected: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("circular dependency detected: %s", e.Message)
}

func (e *CircularDependencyError) Unwrap() error {
	return e.Err
}

// NewCircularDependencyError creates a new CircularDependencyError
func NewCircularDependencyError(message string, branches []string, currentPR, conflictingPR int, err error) *CircularDependencyError {
	return &CircularDependencyError{
		Message:       message,
		Branches:      branches,
		CurrentPR:     currentPR,
		ConflictingPR: conflictingPR,
		Err:           err,
	}
}

// IsCircularDependencyError checks if an error is a CircularDependencyError
func IsCircularDependencyError(err error) bool {
	_, ok := err.(*CircularDependencyError)
	return ok
}

// InvalidBaseError represents an invalid base branch error in stacked PRs
type InvalidBaseError struct {
	Message    string
	BaseBranch string   // The invalid base branch
	ValidBases []string // List of valid base branches (if available)
	Err        error
}

func (e *InvalidBaseError) Error() string {
	if len(e.ValidBases) > 0 {
		validList := ""
		for i, base := range e.ValidBases {
			if i > 0 {
				validList += ", "
			}
			validList += base
		}
		if e.Err != nil {
			return fmt.Sprintf("invalid base branch: %s (branch: %s, valid: %s): %v", e.Message, e.BaseBranch, validList, e.Err)
		}
		return fmt.Sprintf("invalid base branch: %s (branch: %s, valid: %s)", e.Message, e.BaseBranch, validList)
	}
	if e.Err != nil {
		return fmt.Sprintf("invalid base branch: %s (branch: %s): %v", e.Message, e.BaseBranch, e.Err)
	}
	return fmt.Sprintf("invalid base branch: %s (branch: %s)", e.Message, e.BaseBranch)
}

func (e *InvalidBaseError) Unwrap() error {
	return e.Err
}

// NewInvalidBaseError creates a new InvalidBaseError
func NewInvalidBaseError(message string, baseBranch string, validBases []string, err error) *InvalidBaseError {
	return &InvalidBaseError{
		Message:    message,
		BaseBranch: baseBranch,
		ValidBases: validBases,
		Err:        err,
	}
}

// IsInvalidBaseError checks if an error is an InvalidBaseError
func IsInvalidBaseError(err error) bool {
	_, ok := err.(*InvalidBaseError)
	return ok
}

// ParentPRConflictError represents a conflict with the parent PR in stacked workflow
type ParentPRConflictError struct {
	Message    string
	ParentPR   int    // Parent PR number
	ParentState string // Parent PR state (closed, merged, etc.)
	Reason     string // Reason for the conflict
	Err        error
}

func (e *ParentPRConflictError) Error() string {
	if e.Reason != "" {
		if e.Err != nil {
			return fmt.Sprintf("parent PR conflict: %s (PR #%d is %s: %s): %v", e.Message, e.ParentPR, e.ParentState, e.Reason, e.Err)
		}
		return fmt.Sprintf("parent PR conflict: %s (PR #%d is %s: %s)", e.Message, e.ParentPR, e.ParentState, e.Reason)
	}
	if e.Err != nil {
		return fmt.Sprintf("parent PR conflict: %s (PR #%d is %s): %v", e.Message, e.ParentPR, e.ParentState, e.Err)
	}
	return fmt.Sprintf("parent PR conflict: %s (PR #%d is %s)", e.Message, e.ParentPR, e.ParentState)
}

func (e *ParentPRConflictError) Unwrap() error {
	return e.Err
}

// NewParentPRConflictError creates a new ParentPRConflictError
func NewParentPRConflictError(message string, parentPR int, parentState, reason string, err error) *ParentPRConflictError {
	return &ParentPRConflictError{
		Message:     message,
		ParentPR:    parentPR,
		ParentState: parentState,
		Reason:      reason,
		Err:         err,
	}
}

// IsParentPRConflictError checks if an error is a ParentPRConflictError
func IsParentPRConflictError(err error) bool {
	_, ok := err.(*ParentPRConflictError)
	return ok
}

// StackingError represents a general stacking-related error
type StackingError struct {
	Message      string
	CurrentBranch string // Current branch name
	BaseBranch   string // Base branch name
	Operation    string // Operation being performed (create, update, etc.)
	Context      map[string]interface{} // Additional context
	Err          error
}

func (e *StackingError) Error() string {
	contextStr := ""
	if len(e.Context) > 0 {
		contextStr = " ("
		first := true
		for k, v := range e.Context {
			if !first {
				contextStr += ", "
			}
			contextStr += fmt.Sprintf("%s: %v", k, v)
			first = false
		}
		contextStr += ")"
	}

	if e.Operation != "" {
		if e.Err != nil {
			return fmt.Sprintf("stacking error during %s: %s [%s → %s]%s: %v", e.Operation, e.Message, e.CurrentBranch, e.BaseBranch, contextStr, e.Err)
		}
		return fmt.Sprintf("stacking error during %s: %s [%s → %s]%s", e.Operation, e.Message, e.CurrentBranch, e.BaseBranch, contextStr)
	}
	if e.Err != nil {
		return fmt.Sprintf("stacking error: %s [%s → %s]%s: %v", e.Message, e.CurrentBranch, e.BaseBranch, contextStr, e.Err)
	}
	return fmt.Sprintf("stacking error: %s [%s → %s]%s", e.Message, e.CurrentBranch, e.BaseBranch, contextStr)
}

func (e *StackingError) Unwrap() error {
	return e.Err
}

// WithContext adds context to a StackingError
func (e *StackingError) WithContext(key string, value interface{}) *StackingError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewStackingError creates a new StackingError
func NewStackingError(message string, currentBranch, baseBranch, operation string, err error) *StackingError {
	return &StackingError{
		Message:       message,
		CurrentBranch: currentBranch,
		BaseBranch:    baseBranch,
		Operation:     operation,
		Context:       make(map[string]interface{}),
		Err:           err,
	}
}

// IsStackingError checks if an error is a StackingError
func IsStackingError(err error) bool {
	_, ok := err.(*StackingError)
	return ok
}
