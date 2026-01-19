package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// GetAliasName determines the name under which the utility was launched
func GetAliasName() string {
	execPath := os.Args[0]

	// Get only filename without path
	execName := filepath.Base(execPath)

	// Remove extension if present
	execName = strings.TrimSuffix(execName, ".exe")

	return execName
}
