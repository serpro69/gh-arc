package github

import (
	"net/http"
	"strconv"
	"time"
)

// RateLimitInfo contains information about GitHub API rate limits
type RateLimitInfo struct {
	Limit     int       // Maximum number of requests per hour
	Remaining int       // Number of requests remaining in the current window
	Reset     time.Time // Time when the rate limit resets
}

// ParseRateLimitHeaders extracts rate limit information from HTTP response headers
// GitHub returns X-RateLimit-* headers with rate limit information
func ParseRateLimitHeaders(headers http.Header) *RateLimitInfo {
	info := &RateLimitInfo{}

	// Parse X-RateLimit-Limit
	if limit := headers.Get("X-RateLimit-Limit"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			info.Limit = val
		}
	}

	// Parse X-RateLimit-Remaining
	if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			info.Remaining = val
		}
	}

	// Parse X-RateLimit-Reset (Unix timestamp)
	if reset := headers.Get("X-RateLimit-Reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			info.Reset = time.Unix(val, 0)
		}
	}

	return info
}

// IsRateLimited checks if the rate limit has been exceeded
func (r *RateLimitInfo) IsRateLimited() bool {
	return r.Remaining == 0
}

// TimeUntilReset returns the duration until the rate limit resets
func (r *RateLimitInfo) TimeUntilReset() time.Duration {
	return time.Until(r.Reset)
}

// String returns a string representation of the rate limit info
func (r *RateLimitInfo) String() string {
	if r == nil {
		return "no rate limit info"
	}
	return "rate limit: " + strconv.Itoa(r.Remaining) + "/" + strconv.Itoa(r.Limit) +
		" (resets at " + r.Reset.Format(time.RFC3339) + ")"
}
