package lint

import (
	_ "embed"
)

//go:embed default-mega-linter.yml
var defaultMegaLinterConfig []byte

// GetDefaultMegaLinterConfig returns the embedded default mega-linter config as a byte slice
// This can be used for displaying the default config or other operations
func GetDefaultMegaLinterConfig() []byte {
	return defaultMegaLinterConfig
}
