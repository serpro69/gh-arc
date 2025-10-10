package cmd

import (
	"strings"
	"testing"
)

func TestAuthCommand(t *testing.T) {
	t.Run("command initialization", func(t *testing.T) {
		if authCmd.Use != "auth" {
			t.Errorf("Expected Use to be 'auth', got '%s'", authCmd.Use)
		}

		if authCmd.Short == "" {
			t.Error("Expected Short description to be set")
		}

		if authCmd.Long == "" {
			t.Error("Expected Long description to be set")
		}

		if authCmd.RunE == nil {
			t.Error("Expected RunE to be set")
		}
	})

	t.Run("auth command is registered", func(t *testing.T) {
		// Check if auth command is added to root
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == "auth" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected auth command to be registered with root command")
		}
	})
}

func TestOutputAuthStatus(t *testing.T) {
	tests := []struct {
		name           string
		status         AuthStatus
		jsonMode       bool
		expectError    bool
		expectContains []string
	}{
		{
			name: "authenticated user plain text",
			status: AuthStatus{
				Authenticated: true,
				User:          "testuser",
			},
			jsonMode:    false,
			expectError: false,
			expectContains: []string{
				"✓",
				"testuser",
			},
		},
		{
			name: "authenticated user JSON",
			status: AuthStatus{
				Authenticated: true,
				User:          "testuser",
			},
			jsonMode:    true,
			expectError: false,
			expectContains: []string{
				`"authenticated": true`,
				`"user": "testuser"`,
			},
		},
		{
			name: "not authenticated plain text",
			status: AuthStatus{
				Authenticated: false,
				Error:         "auth failed",
			},
			jsonMode:    false,
			expectError: true,
			expectContains: []string{
				"✗",
				"Not authenticated",
				"gh auth login",
			},
		},
		{
			name: "not authenticated JSON",
			status: AuthStatus{
				Authenticated: false,
				Error:         "auth failed",
			},
			jsonMode:    true,
			expectError: true,
			expectContains: []string{
				`"authenticated": false`,
				`"error": "auth failed"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set JSON mode for this test
			originalJSON := jsonOut
			jsonOut = tt.jsonMode
			defer func() { jsonOut = originalJSON }()

			// We can't easily capture stdout in this test without more setup
			// So we'll just test that the function returns the right error
			err := outputAuthStatus(tt.status)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), "not authenticated") {
					t.Errorf("Expected error to contain 'not authenticated', got: %v", err)
				}
			}
		})
	}
}

func TestAuthStatus(t *testing.T) {
	t.Run("authenticated status", func(t *testing.T) {
		status := AuthStatus{
			Authenticated: true,
			User:          "testuser",
		}

		if !status.Authenticated {
			t.Error("Expected status to be authenticated")
		}

		if status.User != "testuser" {
			t.Errorf("Expected user 'testuser', got '%s'", status.User)
		}

		if status.Error != "" {
			t.Errorf("Expected no error for authenticated user, got '%s'", status.Error)
		}
	})

	t.Run("not authenticated status", func(t *testing.T) {
		status := AuthStatus{
			Authenticated: false,
			Error:         "authentication failed",
		}

		if status.Authenticated {
			t.Error("Expected status to not be authenticated")
		}

		if status.User != "" {
			t.Errorf("Expected no user, got '%s'", status.User)
		}

		if status.Error != "authentication failed" {
			t.Errorf("Expected error 'authentication failed', got '%s'", status.Error)
		}
	})
}
