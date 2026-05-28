// Package config handles reading and writing the slap configuration file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"gopkg.in/yaml.v3"
)

// Config represents the slap configuration stored in ~/.config/slap/config.yaml.
type Config struct {
	RepoURL   string `yaml:"repo_url"`
	Branch    string `yaml:"branch"`
	TargetDir string `yaml:"target_dir"`
}

const (
	// SlapDir is the slap config directory path (may contain ~).
	SlapDir = "~/.config/slap"
	// ConfigFile is the default config file path.
	ConfigFile = SlapDir + "/config.yaml"
	// ManifestFile is the default manifest file path.
	ManifestFile = SlapDir + "/manifest.json"
)

// expandHome replaces a leading ~/ with the current user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// Load reads a Config from the given YAML file path.
// Returns an error if the file does not exist or cannot be parsed.
func Load(path string) (*Config, error) {
	expanded := expandHome(path)
	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config atomically to the given path.
// It creates the parent directory if it does not exist.
func (c *Config) Save(path string) error {
	expanded := expandHome(path)
	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Atomic write: temp file + rename
	tmp, err := os.CreateTemp(dir, "config-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, expanded); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// ApplyFlagOverrides sets non-empty flag values on the config.
// Empty strings are ignored so that defaults from the config file are preserved.
func (c *Config) ApplyFlagOverrides(repo, branch, targetDir string) {
	if repo != "" {
		c.RepoURL = repo
	}
	if branch != "" {
		c.Branch = branch
	}
	if targetDir != "" {
		c.TargetDir = targetDir
	}
}

// ValidateRepoAccess checks that a git repo is reachable at the given URL
// by performing an ls-remote equivalent. It does not clone the repo.
func ValidateRepoAccess(url, branch string) error {
	if url == "" {
		return errors.New("repo URL cannot be empty")
	}

	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	if _, err := remote.List(&git.ListOptions{}); err != nil {
		return fmt.Errorf("repo not accessible at %s: %w", url, err)
	}

	return nil
}
