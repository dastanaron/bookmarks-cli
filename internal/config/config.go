package config

import (
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	DBPath string
}

// NewConfig creates a new configuration with defaults
func NewConfig() *Config {
	return &Config{
		DBPath: getDefaultDBPath(),
	}
}

// WithDBPath sets a custom database path
func (c *Config) WithDBPath(path string) *Config {
	c.DBPath = path
	return c
}

func getDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "bookmarks.db"
	}
	return filepath.Join(homeDir, ".bookmarks", "bookmarks.db")
}
