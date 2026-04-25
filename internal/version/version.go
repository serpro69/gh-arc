package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	Version   = "dev"
	GitCommit = ""
	BuildDate = ""
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if GitCommit == "" && len(s.Value) >= 7 {
				GitCommit = s.Value[:7]
			}
		case "vcs.time":
			if BuildDate == "" {
				BuildDate = s.Value
			}
		}
	}
}

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
