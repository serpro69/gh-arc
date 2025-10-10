package version

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// Version information. These variables are set at build time using ldflags.
var (
	// Version is the semantic version (e.g., "1.0.0")
	Version = "dev"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// BuildDate is the build date in RFC3339 format
	BuildDate = "unknown"
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
	// Platform is the OS/Arch combination
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// Info represents version information
type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
}

// GetVersion returns the current version information
func GetVersion() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  Platform,
	}
}

// String returns a human-readable version string
func (i Info) String() string {
	return fmt.Sprintf("gh-arc version %s\ncommit: %s\nbuilt: %s\ngo: %s\nplatform: %s",
		i.Version,
		i.GitCommit,
		i.BuildDate,
		i.GoVersion,
		i.Platform,
	)
}

// JSON returns the version information as JSON
func (i Info) JSON() (string, error) {
	data, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal version info: %w", err)
	}
	return string(data), nil
}
