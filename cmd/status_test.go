package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

func TestStatusCmd_NoConfig(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := NewRootCmd()
	root.SetArgs([]string{"status"})
	err := root.Execute()
	if err == nil {
		t.Fatal("status without config should have failed, got nil")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error should mention 'not configured', got: %v", err)
	}
}

func TestStatusCmd_UpToDate(t *testing.T) {
	homeDir := setupSyncTest(t, []string{"skill-a"})

	// Run sync first to install the skill and create manifest.
	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Now run status — skill-a should be up-to-date.
	root = NewRootCmd()
	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	// We can't easily capture output here (it goes to stdout),
	// but the command didn't error — that's the basic check.
	// We verify by checking that manifest and local state match.
	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	if !m.HasSkill("skill-a") {
		t.Error("manifest should have skill-a")
	}
}

func TestStatusCmd_MissingLocally(t *testing.T) {
	homeDir := setupSyncTest(t, []string{"skill-a"})
	targetDir := filepath.Join(homeDir, ".config", "opencode", "skills")

	// Run sync to install and create manifest.
	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Delete the skill directory locally.
	if err := os.RemoveAll(filepath.Join(targetDir, "skill-a")); err != nil {
		t.Fatalf("removing skill-a: %v", err)
	}

	// Status should still succeed (it reports "missing" but doesn't error).
	root = NewRootCmd()
	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status with missing skill should not error: %v", err)
	}
}

func TestStatusCmd_NewSkillInRepo(t *testing.T) {
	homeDir := setupSyncTest(t, []string{"skill-a"})
	targetDir := filepath.Join(homeDir, ".config", "opencode", "skills")

	// Run initial sync.
	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Create a second repo that has an additional skill.
	newRepoDir := t.TempDir()
	createLocalRepo(t, newRepoDir, "main", []string{"skill-a", "skill-b"})

	// Update config to point to new repo.
	cfg := &config.Config{
		RepoURL:   "file://" + newRepoDir,
		Branch:    "main",
		TargetDir: "~/.config/opencode/skills",
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving updated config: %v", err)
	}

	// Status should detect skill-b as "new".
	root = NewRootCmd()
	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status should not error with new repo skills: %v", err)
	}

	// Verify skill-b is NOT yet installed.
	if _, err := os.Stat(filepath.Join(targetDir, "skill-b", "skill.yaml")); !os.IsNotExist(err) {
		t.Error("skill-b should not be installed yet (status only, not sync)")
	}
}

func TestStatusCmd_LocallyModified(t *testing.T) {
	homeDir := setupSyncTest(t, []string{"skill-a"})
	targetDir := filepath.Join(homeDir, ".config", "opencode", "skills")

	root := NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Modify a local skill file (this changes the local tree SHA).
	skillFile := filepath.Join(targetDir, "skill-a", "skill.yaml")
	if err := os.WriteFile(skillFile, []byte("modified: true\n"), 0644); err != nil {
		t.Fatalf("modifying skill file: %v", err)
	}

	// Status should still succeed (detects local modification).
	root = NewRootCmd()
	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status after local modification should not error: %v", err)
	}
}
