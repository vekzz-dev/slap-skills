package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &Config{
		RepoURL:   "https://github.com/user/skills.git",
		Branch:    "main",
		TargetDir: "~/.config/opencode/skills",
	}

	// Save
	if err := original.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.RepoURL != original.RepoURL {
		t.Errorf("RepoURL = %q, want %q", loaded.RepoURL, original.RepoURL)
	}
	if loaded.Branch != original.Branch {
		t.Errorf("Branch = %q, want %q", loaded.Branch, original.Branch)
	}
	if loaded.TargetDir != original.TargetDir {
		t.Errorf("TargetDir = %q, want %q", loaded.TargetDir, original.TargetDir)
	}
}

func TestLoadReturnsErrorOnMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("Load: expected error for missing file, got nil")
	}
}

func TestLoadReturnsErrorOnInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("{{invalid yaml}}"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load: expected error for invalid YAML, got nil")
	}
}

func TestApplyFlagOverrides(t *testing.T) {
	tests := []struct {
		name            string
		initial         Config
		repo, branch, targetDir string
		wantRepo, wantBranch, wantTarget string
	}{
		{
			name:       "override all fields",
			initial:    Config{RepoURL: "old", Branch: "old", TargetDir: "old"},
			repo:       "new-url", branch: "new-branch", targetDir: "new-dir",
			wantRepo:   "new-url", wantBranch: "new-branch", wantTarget: "new-dir",
		},
		{
			name:       "empty overrides preserve defaults",
			initial:    Config{RepoURL: "old", Branch: "old", TargetDir: "old"},
			repo:       "", branch: "", targetDir: "",
			wantRepo:   "old", wantBranch: "old", wantTarget: "old",
		},
		{
			name:       "partial override only repo",
			initial:    Config{RepoURL: "old", Branch: "main", TargetDir: "skills"},
			repo:       "new-url", branch: "", targetDir: "",
			wantRepo:   "new-url", wantBranch: "main", wantTarget: "skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.initial
			c.ApplyFlagOverrides(tt.repo, tt.branch, tt.targetDir)
			if c.RepoURL != tt.wantRepo {
				t.Errorf("RepoURL = %q, want %q", c.RepoURL, tt.wantRepo)
			}
			if c.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", c.Branch, tt.wantBranch)
			}
			if c.TargetDir != tt.wantTarget {
				t.Errorf("TargetDir = %q, want %q", c.TargetDir, tt.wantTarget)
			}
		})
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	// Use a nested path that doesn't exist yet
	path := filepath.Join(dir, "nested", "subdir", "config.yaml")

	cfg := &Config{RepoURL: "https://example.com/repo.git"}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save with nested dirs failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Save: config file was not created")
	}
}

func TestValidateRepoAccessWithBadURL(t *testing.T) {
	// This test attempts to reach a network resource and may be slow.
	// Skip in short mode.
	if testing.Short() {
		t.Skip("Skipping network-dependent test in short mode")
	}

	err := ValidateRepoAccess("https://invalid.repo.url.that.does.not.exist/foo.git", "main")
	if err == nil {
		t.Fatal("ValidateRepoAccess: expected error for unreachable URL, got nil")
	}
}
