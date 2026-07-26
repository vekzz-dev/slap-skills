package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSourceCRUD tests CreateSource, ReadSource, DeleteSource, and ListSources.
func TestSourceCRUD(t *testing.T) {
	// Use a temp dir as the slap config home
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Initially no sources
	sources, err := ListSources()
	if err != nil {
		t.Fatalf("ListSources on empty dir: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}

	// Create a source
	sc := SourceConfig{
		Alias:  "work",
		URL:    "https://github.com/work/skills.git",
		Branch: "main",
	}
	if err := CreateSource("work", sc); err != nil {
		t.Fatalf("CreateSource failed: %v", err)
	}

	// List should find it
	sources, err = ListSources()
	if err != nil {
		t.Fatalf("ListSources after create: %v", err)
	}
	if len(sources) != 1 || sources[0] != "work" {
		t.Errorf("ListSources = %v, want [work]", sources)
	}

	// Read should return the same data
	got, err := ReadSource("work")
	if err != nil {
		t.Fatalf("ReadSource failed: %v", err)
	}
	if got.URL != sc.URL {
		t.Errorf("URL = %q, want %q", got.URL, sc.URL)
	}
	if got.Branch != sc.Branch {
		t.Errorf("Branch = %q, want %q", got.Branch, sc.Branch)
	}
	if got.Alias != sc.Alias {
		t.Errorf("Alias = %q, want %q", got.Alias, sc.Alias)
	}

	// Delete should succeed
	if err := DeleteSource("work"); err != nil {
		t.Fatalf("DeleteSource failed: %v", err)
	}

	// List should be empty again
	sources, err = ListSources()
	if err != nil {
		t.Fatalf("ListSources after delete: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources after delete, got %d", len(sources))
	}

	// Read deleted source should fail
	if _, err := ReadSource("work"); err == nil {
		t.Error("expected error reading deleted source")
	}
}

// TestCreateSourceCreatesDir tests that CreateSource creates the sources dir.
func TestCreateSourceCreatesDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sc := SourceConfig{Alias: "test", URL: "https://example.com/repo.git", Branch: "main"}
	if err := CreateSource("test", sc); err != nil {
		t.Fatalf("CreateSource failed: %v", err)
	}

	sourcesDir := SourcesDir()
	if _, err := os.Stat(sourcesDir); os.IsNotExist(err) {
		t.Error("SourcesDir was not created")
	}
}

// TestListSourcesIgnoresNonYaml tests that ListSources skips non-.yaml files.
func TestListSourcesIgnoresNonYaml(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create a non-YAML file in the sources dir
	sourcesDir := SourcesDir()
	if err := os.MkdirAll(sourcesDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcesDir, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a valid source
	sc := SourceConfig{Alias: "valid", URL: "https://example.com/r.git", Branch: "main"}
	if err := CreateSource("valid", sc); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	sources, err := ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 1 || sources[0] != "valid" {
		t.Errorf("ListSources = %v, want [valid]", sources)
	}
}

// TestMigrateConfigNoOpOnNewFormat tests that MigrateConfig is a no-op when
// sources already exist.
func TestMigrateConfigNoOpOnNewFormat(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create a config already in new format (no repo_url, sources set)
	cfg := &Config{
		TargetDir: "~/.config/opencode/skills",
		Sources:   []string{"default"},
	}
	if err := cfg.Save(ConfigFile); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	// Create sources dir so migration detects it's already done
	if err := os.MkdirAll(SourcesDir(), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := MigrateConfig(); err != nil {
		t.Fatalf("MigrateConfig should be no-op: %v", err)
	}

	// Config should be unchanged
	got, err := Load(ConfigFile)
	if err != nil {
		t.Fatalf("Load after migration: %v", err)
	}
	if got.RepoURL != "" {
		t.Errorf("RepoURL should remain empty, got %q", got.RepoURL)
	}
}

// TestMigrateConfigWithOldFormat tests full migration from old to new format.
func TestMigrateConfigWithOldFormat(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create an old-style config
	oldCfg := &Config{
		RepoURL:   "https://github.com/user/skills.git",
		Branch:    "main",
		TargetDir: "~/.config/opencode/skills",
	}
	if err := oldCfg.Save(ConfigFile); err != nil {
		t.Fatalf("Save old config: %v", err)
	}

	// Run migration
	if err := MigrateConfig(); err != nil {
		t.Fatalf("MigrateConfig failed: %v", err)
	}

	// Verify backup exists
	bakPath := filepath.Join(homeDir, ".config", "slap", "config.yaml.bak")
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		t.Fatal("backup file not found")
	}

	// Verify default source was created
	source, err := ReadSource("default")
	if err != nil {
		t.Fatalf("ReadSource(default) failed: %v", err)
	}
	if source.URL != "https://github.com/user/skills.git" {
		t.Errorf("source URL = %q, want %q", source.URL, "https://github.com/user/skills.git")
	}
	if source.Branch != "main" {
		t.Errorf("source Branch = %q, want %q", source.Branch, "main")
	}

	// Verify config was rewritten
	cfg, err := Load(ConfigFile)
	if err != nil {
		t.Fatalf("Load migrated config: %v", err)
	}
	if cfg.RepoURL != "" {
		t.Errorf("RepoURL should be empty after migration, got %q", cfg.RepoURL)
	}
	if cfg.Branch != "" {
		t.Errorf("Branch should be empty after migration, got %q", cfg.Branch)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0] != "default" {
		t.Errorf("Sources = %v, want [default]", cfg.Sources)
	}
	if cfg.TargetDir != "~/.config/opencode/skills" {
		t.Errorf("TargetDir should be preserved = %q", cfg.TargetDir)
	}
}

// TestMigrateConfigIdempotent tests that running MigrateConfig twice is safe.
func TestMigrateConfigIdempotent(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	oldCfg := &Config{
		RepoURL:   "https://github.com/user/skills.git",
		Branch:    "main",
		TargetDir: "~/.config/opencode/skills",
	}
	if err := oldCfg.Save(ConfigFile); err != nil {
		t.Fatalf("Save old config: %v", err)
	}

	if err := MigrateConfig(); err != nil {
		t.Fatalf("First MigrateConfig failed: %v", err)
	}

	// Second call should be a no-op (sources dir already exists)
	if err := MigrateConfig(); err != nil {
		t.Fatalf("Second MigrateConfig should be no-op: %v", err)
	}
}

// TestMigrateConfigNoRepoURL tests that no migration happens when repo_url is empty.
func TestMigrateConfigNoRepoURL(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := &Config{
		TargetDir: "~/.config/opencode/skills",
	}
	if err := cfg.Save(ConfigFile); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	if err := MigrateConfig(); err != nil {
		t.Fatalf("MigrateConfig should be no-op: %v", err)
	}

	// No backup file should exist
	bakPath := filepath.Join(homeDir, ".config", "slap", "config.yaml.bak")
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Error("backup should not exist when no migration was needed")
	}
}

// TestMigrateConfigNoConfig tests that MigrateConfig handles missing config.
func TestMigrateConfigNoConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := MigrateConfig(); err != nil {
		t.Fatalf("MigrateConfig should be no-op when config doesn't exist: %v", err)
	}
}
