package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

// setupSyncTest creates a local source repo, writes a slap config pointing to
// it via a file:// URL, and returns the home directory.  Callers should set
// HOME to the returned directory.
func setupSyncTest(t *testing.T, skills []string) (homeDir string) {
	t.Helper()

	repoDir := t.TempDir()
	createLocalRepo(t, repoDir, "main", skills)

	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := &config.Config{
		RepoURL:   "file://" + repoDir,
		Branch:    "main",
		TargetDir: "~/.config/opencode/skills",
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	return homeDir
}

// installAllSkills runs slap install --all to install all available skills.
func installAllSkills(t *testing.T) {
	t.Helper()
	root := NewRootCmd()
	root.SetArgs([]string{"install", "--all"})
	if err := root.Execute(); err != nil {
		t.Fatalf("install --all failed: %v", err)
	}
}

func TestSyncCmd_BasicFlow(t *testing.T) {
	homeDir := setupSyncTest(t, []string{"skill-a", "skill-b"})
	targetDir := filepath.Join(homeDir, ".config", "opencode", "skills")

	// Install skills first
	installAllSkills(t)

	// Sync updates existing skills
	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Verify skill directories were created.
	for _, skill := range []string{"skill-a", "skill-b"} {
		skillPath := filepath.Join(targetDir, skill, "skill.yaml")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			t.Errorf("missing skill file: %s", skillPath)
		}
	}

	// Verify manifest was created with the skills.
	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	if !m.HasSkill("skill-a") {
		t.Error("manifest missing skill-a")
	}
	if !m.HasSkill("skill-b") {
		t.Error("manifest missing skill-b")
	}
	if m.SourceRepo == "" {
		t.Error("SourceRepo should be set in manifest")
	}
}

func TestSyncCmd_NoConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	err := root.Execute()
	if err == nil {
		t.Fatal("sync without config should have failed, got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error should mention 'not configured', got: %v", err)
	}
}

func TestSyncCmd_Idempotent(t *testing.T) {
	setupSyncTest(t, []string{"skill-a"})

	// Install skills first
	installAllSkills(t)

	// First sync.
	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Second sync should be a no-op.
	root = NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
}

func TestSyncCmd_Prune(t *testing.T) {
	homeDir := setupSyncTest(t, []string{"skill-a", "skill-b"})
	targetDir := filepath.Join(homeDir, ".config", "opencode", "skills")
	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")

	// Install skills first
	installAllSkills(t)

	// Verify both exist.
	if _, err := os.Stat(filepath.Join(targetDir, "skill-b", "skill.yaml")); os.IsNotExist(err) {
		t.Fatal("skill-b should exist after install")
	}

	// Now we need a new repo that only has skill-a.
	newRepoDir := t.TempDir()
	createLocalRepo(t, newRepoDir, "main", []string{"skill-a"})

	// Update config to point to new repo.
	cfg := &config.Config{
		RepoURL:   "file://" + newRepoDir,
		Branch:    "main",
		TargetDir: "~/.config/opencode/skills",
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving updated config: %v", err)
	}

	// Sync with --prune.
	root := NewRootCmd()
	root.SetArgs([]string{"sync", "--prune"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync --prune failed: %v", err)
	}

	// skill-b should be removed.
	if _, err := os.Stat(filepath.Join(targetDir, "skill-b", "skill.yaml")); !os.IsNotExist(err) {
		t.Error("skill-b should have been pruned")
	}

	// skill-a should still exist.
	if _, err := os.Stat(filepath.Join(targetDir, "skill-a", "skill.yaml")); os.IsNotExist(err) {
		t.Error("skill-a should still exist after prune")
	}

	// Manifest should only have skill-a.
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	if m.HasSkill("skill-b") {
		t.Error("manifest should not have skill-b after prune")
	}
	if !m.HasSkill("skill-a") {
		t.Error("manifest should have skill-a after prune")
	}
}
