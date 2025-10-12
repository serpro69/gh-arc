package github

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	// Note: This test will fail if gh CLI is not authenticated
	// In CI/CD, we'd mock the api.DefaultRESTClient and api.DefaultGraphQLClient
	t.Run("creates client with defaults", func(t *testing.T) {
		client, err := NewClient()

		// We expect an error if gh is not authenticated, which is fine for this test
		if err != nil {
			if !IsAuthenticationError(err) {
				t.Errorf("Expected AuthenticationError, got: %v", err)
			}
			t.Skip("Skipping test - gh CLI not authenticated")
			return
		}

		if client == nil {
			t.Fatal("Expected client to be created, got nil")
		}

		if client.config == nil {
			t.Error("Expected config to be initialized")
		}

		if client.cache == nil {
			t.Error("Expected cache to be initialized")
		}

		if client.restClient == nil {
			t.Error("Expected REST client to be initialized")
		}

		if client.graphqlClient == nil {
			t.Error("Expected GraphQL client to be initialized")
		}

		// Repository may be nil if not in a git repo
		// This is expected behavior
	})

	t.Run("applies options", func(t *testing.T) {
		client, err := NewClient(
			WithMaxRetries(5),
			WithRepository("facebook", "react"),
		)

		if err != nil {
			if !IsAuthenticationError(err) {
				t.Errorf("Expected AuthenticationError, got: %v", err)
			}
			t.Skip("Skipping test - gh CLI not authenticated")
			return
		}

		if client.config.MaxRetries != 5 {
			t.Errorf("Expected MaxRetries=5, got %d", client.config.MaxRetries)
		}

		if client.repo == nil {
			t.Fatal("Expected repository to be set")
		}

		if client.repo.Owner != "facebook" || client.repo.Name != "react" {
			t.Errorf("Expected repo facebook/react, got %s/%s", client.repo.Owner, client.repo.Name)
		}
	})
}

func TestCurrentUser(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		if !IsAuthenticationError(err) {
			t.Errorf("Expected AuthenticationError, got: %v", err)
		}
		t.Skip("Skipping test - gh CLI not authenticated")
		return
	}

	ctx := context.Background()
	user, err := client.CurrentUser(ctx)

	if err != nil {
		if !IsAuthenticationError(err) {
			t.Errorf("Expected AuthenticationError, got: %v", err)
		}
		t.Skip("Skipping test - gh CLI not authenticated or network issue")
		return
	}

	if user == nil {
		t.Fatal("Expected user to be returned, got nil")
	}

	if user.Login == "" {
		t.Error("Expected user login to be set")
	}
}

func TestVerifyAuthentication(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		if !IsAuthenticationError(err) {
			t.Errorf("Expected AuthenticationError, got: %v", err)
		}
		t.Skip("Skipping test - gh CLI not authenticated")
		return
	}

	ctx := context.Background()
	err = client.VerifyAuthentication(ctx)

	if err != nil {
		if !IsAuthenticationError(err) {
			t.Errorf("Expected AuthenticationError, got: %v", err)
		}
		t.Skip("Skipping test - gh CLI not authenticated or network issue")
		return
	}
}
