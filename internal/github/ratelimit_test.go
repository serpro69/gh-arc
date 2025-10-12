package github

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRateLimitHeaders(t *testing.T) {
	t.Run("parses all headers", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-RateLimit-Limit", "5000")
		headers.Set("X-RateLimit-Remaining", "4999")
		headers.Set("X-RateLimit-Reset", "1234567890")

		info := ParseRateLimitHeaders(headers)

		if info.Limit != 5000 {
			t.Errorf("Expected Limit=5000, got %d", info.Limit)
		}

		if info.Remaining != 4999 {
			t.Errorf("Expected Remaining=4999, got %d", info.Remaining)
		}

		expectedTime := time.Unix(1234567890, 0)
		if !info.Reset.Equal(expectedTime) {
			t.Errorf("Expected Reset=%v, got %v", expectedTime, info.Reset)
		}
	})

	t.Run("handles missing headers", func(t *testing.T) {
		headers := http.Header{}

		info := ParseRateLimitHeaders(headers)

		if info.Limit != 0 {
			t.Errorf("Expected Limit=0, got %d", info.Limit)
		}

		if info.Remaining != 0 {
			t.Errorf("Expected Remaining=0, got %d", info.Remaining)
		}

		if !info.Reset.IsZero() {
			t.Errorf("Expected Zero time, got %v", info.Reset)
		}
	})

	t.Run("handles invalid values", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-RateLimit-Limit", "invalid")
		headers.Set("X-RateLimit-Remaining", "invalid")
		headers.Set("X-RateLimit-Reset", "invalid")

		info := ParseRateLimitHeaders(headers)

		if info.Limit != 0 {
			t.Errorf("Expected Limit=0 for invalid value, got %d", info.Limit)
		}

		if info.Remaining != 0 {
			t.Errorf("Expected Remaining=0 for invalid value, got %d", info.Remaining)
		}

		if !info.Reset.IsZero() {
			t.Errorf("Expected Zero time for invalid value, got %v", info.Reset)
		}
	})
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		expected  bool
	}{
		{"not rate limited", 100, false},
		{"rate limited", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &RateLimitInfo{
				Limit:     5000,
				Remaining: tt.remaining,
				Reset:     time.Now().Add(1 * time.Hour),
			}

			result := info.IsRateLimited()
			if result != tt.expected {
				t.Errorf("IsRateLimited() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestTimeUntilReset(t *testing.T) {
	resetTime := time.Now().Add(1 * time.Hour)
	info := &RateLimitInfo{
		Limit:     5000,
		Remaining: 0,
		Reset:     resetTime,
	}

	duration := info.TimeUntilReset()

	// Should be close to 1 hour (within 1 second tolerance for test execution time)
	expected := 1 * time.Hour
	tolerance := 1 * time.Second

	if duration < expected-tolerance || duration > expected+tolerance {
		t.Errorf("TimeUntilReset() = %v, expected ~%v", duration, expected)
	}
}

func TestRateLimitInfoString(t *testing.T) {
	t.Run("with rate limit info", func(t *testing.T) {
		resetTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		info := &RateLimitInfo{
			Limit:     5000,
			Remaining: 4999,
			Reset:     resetTime,
		}

		str := info.String()
		if str == "" {
			t.Error("Expected non-empty string representation")
		}

		// Should contain key information
		if len(str) < 10 {
			t.Errorf("String representation seems too short: %s", str)
		}
	})

	t.Run("with nil info", func(t *testing.T) {
		var info *RateLimitInfo
		str := info.String()

		if str != "no rate limit info" {
			t.Errorf("Expected 'no rate limit info', got %s", str)
		}
	})
}
