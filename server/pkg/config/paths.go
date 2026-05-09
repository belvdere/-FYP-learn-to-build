package config

import (
	"os"
	"path/filepath"
)

// GetCacheDir returns the cache directory for FYP models and data.
// Defaults to ~/.cache/fyp or platform equivalent.
func GetCacheDir() (string, error) {
	userCache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(userCache, "fyp")
	return cacheDir, nil
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// GetDBPath returns the path to the workspace index database.
// workspaceRoot is the root directory of the workspace being indexed.
func GetDBPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".fyp", "index.db")
}
