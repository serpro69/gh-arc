package version

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	// Save original values
	origVersion := Version
	origGitCommit := GitCommit
	origBuildDate := BuildDate
	origGoVersion := GoVersion
	origPlatform := Platform

	// Set test values
	Version = "1.0.0"
	GitCommit = "abc123"
	BuildDate = "2024-01-01T00:00:00Z"

	// Restore original values after test
	defer func() {
		Version = origVersion
		GitCommit = origGitCommit
		BuildDate = origBuildDate
		GoVersion = origGoVersion
		Platform = origPlatform
	}()

	info := GetVersion()

	if info.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got '%s'", info.Version)
	}

	if info.GitCommit != "abc123" {
		t.Errorf("Expected GitCommit to be 'abc123', got '%s'", info.GitCommit)
	}

	if info.BuildDate != "2024-01-01T00:00:00Z" {
		t.Errorf("Expected BuildDate to be '2024-01-01T00:00:00Z', got '%s'", info.BuildDate)
	}

	if info.GoVersion == "" {
		t.Error("Expected GoVersion to be set")
	}

	if info.Platform == "" {
		t.Error("Expected Platform to be set")
	}
}

func TestInfo_String(t *testing.T) {
	info := Info{
		Version:   "1.2.3",
		GitCommit: "def456",
		BuildDate: "2024-02-01T12:00:00Z",
		GoVersion: "go1.21.0",
		Platform:  "linux/amd64",
	}

	output := info.String()

	expectedParts := []string{
		"gh-arc version 1.2.3",
		"commit: def456",
		"built: 2024-02-01T12:00:00Z",
		"go: go1.21.0",
		"platform: linux/amd64",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected output to contain '%s', got:\n%s", part, output)
		}
	}
}

func TestInfo_JSON(t *testing.T) {
	info := Info{
		Version:   "2.0.0",
		GitCommit: "xyz789",
		BuildDate: "2024-03-01T15:30:00Z",
		GoVersion: "go1.22.0",
		Platform:  "darwin/arm64",
	}

	jsonStr, err := info.JSON()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify it's valid JSON by unmarshaling
	var decoded Info
	err = json.Unmarshal([]byte(jsonStr), &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check all fields match
	if decoded.Version != info.Version {
		t.Errorf("Expected Version '%s', got '%s'", info.Version, decoded.Version)
	}
	if decoded.GitCommit != info.GitCommit {
		t.Errorf("Expected GitCommit '%s', got '%s'", info.GitCommit, decoded.GitCommit)
	}
	if decoded.BuildDate != info.BuildDate {
		t.Errorf("Expected BuildDate '%s', got '%s'", info.BuildDate, decoded.BuildDate)
	}
	if decoded.GoVersion != info.GoVersion {
		t.Errorf("Expected GoVersion '%s', got '%s'", info.GoVersion, decoded.GoVersion)
	}
	if decoded.Platform != info.Platform {
		t.Errorf("Expected Platform '%s', got '%s'", info.Platform, decoded.Platform)
	}
}

func TestInfo_JSON_Format(t *testing.T) {
	info := Info{
		Version:   "1.0.0",
		GitCommit: "abc",
		BuildDate: "2024-01-01",
		GoVersion: "go1.21",
		Platform:  "linux/amd64",
	}

	jsonStr, err := info.JSON()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that JSON is pretty-printed (has newlines and indentation)
	if !strings.Contains(jsonStr, "\n") {
		t.Error("Expected JSON to be pretty-printed with newlines")
	}

	if !strings.Contains(jsonStr, "  ") {
		t.Error("Expected JSON to be indented")
	}
}

func TestDefaultValues(t *testing.T) {
	// Test that default values are reasonable
	// (This test assumes we haven't set custom values via ldflags in test)
	if GoVersion == "" {
		t.Error("Expected GoVersion to have a default value")
	}

	if Platform == "" {
		t.Error("Expected Platform to have a default value")
	}

	// Check Platform format
	if !strings.Contains(Platform, "/") {
		t.Errorf("Expected Platform to be in 'os/arch' format, got '%s'", Platform)
	}
}
