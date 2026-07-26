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

// SourceConfig represents a single source configuration stored in
// ~/.config/slap/sources/<alias>.yaml.
type SourceConfig struct {
	Alias  string `yaml:"alias"`
	URL    string `yaml:"url"`
	Branch string `yaml:"branch"`
}

// Config represents the slap configuration stored in ~/.config/slap/config.yaml.
type Config struct {
	RepoURL   string   `yaml:"repo_url"`
	Branch    string   `yaml:"branch"`
	TargetDir string   `yaml:"target_dir"`
	Sources   []string `yaml:"sources"`
}

const (
	// SlapDir is the slap config directory path (may contain ~).
	SlapDir = "~/.config/slap"
	// ConfigFile is the default config file path.
	ConfigFile = SlapDir + "/config.yaml"
	// ManifestFile is the default manifest file path.
	ManifestFile = SlapDir + "/manifest.json"
	// SourcesDirName is the name of the sources directory under SlapDir.
	SourcesDirName = "sources"
)

// SourcesDir returns the expanded path to the sources directory.
func SourcesDir() string {
	return filepath.Join(expandHome(SlapDir), SourcesDirName)
}

// sourcePath returns the full path to a source YAML file for the given alias.
func sourcePath(alias string) string {
	return filepath.Join(SourcesDir(), alias+".yaml")
}

// CreateSource writes a source config file for the given alias.
// It creates the sources directory if it does not exist.
func CreateSource(alias string, sc SourceConfig) error {
	dir := SourcesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating sources directory: %w", err)
	}
	data, err := yaml.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshaling source config: %w", err)
	}
	path := sourcePath(alias)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing source file: %w", err)
	}
	return nil
}

// ReadSource reads a source config file for the given alias.
func ReadSource(alias string) (*SourceConfig, error) {
	data, err := os.ReadFile(sourcePath(alias))
	if err != nil {
		return nil, fmt.Errorf("reading source %q: %w", alias, err)
	}
	var sc SourceConfig
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parsing source %q: %w", alias, err)
	}
	return &sc, nil
}

// DeleteSource removes a source config file for the given alias.
func DeleteSource(alias string) error {
	path := sourcePath(alias)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting source %q: %w", alias, err)
	}
	return nil
}

// ListSources returns the list of source aliases (YAML file names without
// extension) in the sources directory. Returns nil, nil if the directory
// does not exist.
func ListSources() ([]string, error) {
	dir := SourcesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sources directory: %w", err)
	}
	var aliases []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") {
			aliases = append(aliases, strings.TrimSuffix(name, ".yaml"))
		}
	}
	return aliases, nil
}

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

// MigrateConfig detects an old-style config (with repo_url set but no sources
// directory) and migrates it to the multi-source format.
//
// Steps:
//  1. Back up config.yaml → config.yaml.bak
//  2. Create sources/default.yaml with URL and branch from old config
//  3. Rewrite config.yaml: remove repo_url/branch, add sources: ["default"]
//
// Returns nil when no migration is needed (sources dir already exists, or
// repo_url is empty).
func MigrateConfig() error {
	configPath := expandHome(ConfigFile)
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("checking config: %w", err)
	}

	cfg, err := Load(ConfigFile)
	if err != nil {
		return fmt.Errorf("loading config for migration: %w", err)
	}

	// Nothing to migrate if repo_url is not set or sources dir already exists.
	if cfg.RepoURL == "" {
		return nil
	}
	if _, err := os.Stat(SourcesDir()); err == nil {
		return nil
	}

	// Step 1: Back up config.yaml
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config for backup: %w", err)
	}
	bakPath := configPath + ".bak"
	if err := os.WriteFile(bakPath, data, 0644); err != nil {
		return fmt.Errorf("backing up config: %w", err)
	}

	// Step 2: Create sources/default.yaml
	defaultSource := SourceConfig{
		Alias:  "default",
		URL:    cfg.RepoURL,
		Branch: cfg.Branch,
	}
	if err := CreateSource("default", defaultSource); err != nil {
		return fmt.Errorf("creating default source: %w", err)
	}

	// Step 3: Rewrite config — remove repo_url/branch, add sources: ["default"]
	cfg.Sources = []string{"default"}
	cfg.RepoURL = ""
	cfg.Branch = ""
	if err := cfg.Save(ConfigFile); err != nil {
		return fmt.Errorf("saving migrated config: %w", err)
	}

	return nil
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
