package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vekzz-dev/slap-skills/internal/config"
	"github.com/vekzz-dev/slap-skills/internal/manifest"
)

// setupSyncTest creates a local source repo, writes a multi-source slap config
// pointing to it via a file:// URL, and returns the home directory.  Callers
// should set HOME to the returned directory.
func setupSyncTest(t *testing.T, skills []string) (homeDir string) {
	t.Helper()

	repoDir := t.TempDir()
	createLocalRepo(t, repoDir, "main", skills)

	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create multi-source config with a "default" source.
	sourceCfg := config.SourceConfig{
		Alias:  "default",
		URL:    "file://" + repoDir,
		Branch: "main",
	}
	if err := config.CreateSource("default", sourceCfg); err != nil {
		t.Fatalf("creating default source: %v", err)
	}

	cfg := &config.Config{
		TargetDir: "~/.config/opencode/skills",
		Sources:   []string{"default"},
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
	if !m.HasSkill("default", "skill-a") {
		t.Error("manifest missing skill-a (default)")
	}
	if !m.HasSkill("default", "skill-b") {
		t.Error("manifest missing skill-b (default)")
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

func TestSyncCmd_MultiSource(t *testing.T) {
	// Create two repos with different skills.
	repoA := t.TempDir()
	createLocalRepo(t, repoA, "main", []string{"skill-from-a"})

	repoB := t.TempDir()
	createLocalRepo(t, repoB, "main", []string{"skill-from-b"})

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create two sources.
	if err := config.CreateSource("source-a", config.SourceConfig{
		Alias:  "source-a",
		URL:    "file://" + repoA,
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating source-a: %v", err)
	}
	if err := config.CreateSource("source-b", config.SourceConfig{
		Alias:  "source-b",
		URL:    "file://" + repoB,
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating source-b: %v", err)
	}

	cfg := &config.Config{
		TargetDir: "~/.config/opencode/skills",
		Sources:   []string{"source-a", "source-b"},
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// Install all skills.
	root := NewRootCmd()
	root.SetArgs([]string{"install", "--all"})
	if err := root.Execute(); err != nil {
		t.Fatalf("install --all failed: %v", err)
	}

	// Verify both skills are in manifest under their respective sources.
	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	if !m.HasSkill("source-a", "skill-from-a") {
		t.Error("manifest missing skill-from-a (source-a)")
	}
	if !m.HasSkill("source-b", "skill-from-b") {
		t.Error("manifest missing skill-from-b (source-b)")
	}

	// Sync should be a no-op (both skills already installed).
	root = NewRootCmd()
	root.SetArgs([]string{"sync"})
	if err := root.Execute(); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Verify still has both.
	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("reloading manifest: %v", err)
	}
	if !m.HasSkill("source-a", "skill-from-a") {
		t.Error("manifest missing skill-from-a after sync")
	}
	if !m.HasSkill("source-b", "skill-from-b") {
		t.Error("manifest missing skill-from-b after sync")
	}

	// status should show both.
	root = NewRootCmd()
	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	// Remove a skill from source-a, verify source-b is untouched.
	root = NewRootCmd()
	root.SetArgs([]string{"remove", "source-a:skill-from-a"})
	if err := root.Execute(); err != nil {
		t.Fatalf("remove source-a:skill-from-a failed: %v", err)
	}

	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("reloading manifest after remove: %v", err)
	}
	if m.HasSkill("source-a", "skill-from-a") {
		t.Error("skill-from-a should be removed")
	}
	if !m.HasSkill("source-b", "skill-from-b") {
		t.Error("skill-from-b should still exist")
	}
}

func TestSyncCmd_ErrorIsolation(t *testing.T) {
	// Create one valid repo.
	repoA := t.TempDir()
	createLocalRepo(t, repoA, "main", []string{"skill-from-a"})

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Source A — valid repo.
	if err := config.CreateSource("good-source", config.SourceConfig{
		Alias:  "good-source",
		URL:    "file://" + repoA,
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating good-source: %v", err)
	}

	// Source B — unreachable URL (invalid host).
	if err := config.CreateSource("bad-source", config.SourceConfig{
		Alias:  "bad-source",
		URL:    "https://invalid.example.com/unreachable-repo",
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating bad-source: %v", err)
	}

	cfg := &config.Config{
		TargetDir: "~/.config/opencode/skills",
		Sources:   []string{"good-source", "bad-source"},
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// Install from good-source first (directly, not via --all, to avoid
	// install's fail-fast behavior which is by design for interactive use).
	root := NewRootCmd()
	root.SetArgs([]string{"install", "--all", "--source", "good-source"})
	if err := root.Execute(); err != nil {
		t.Fatalf("install --all --source good-source failed: %v", err)
	}

	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}
	if !m.HasSkill("good-source", "skill-from-a") {
		t.Fatal("skill-from-a (good-source) should be installed")
	}

	// Now sync — bad-source should fail, good-source should succeed.
	root = NewRootCmd()
	root.SetArgs([]string{"sync"})
	// sync continues despite errors — it doesn't return an error for per-source failures
	_ = root.Execute()

	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("reloading manifest: %v", err)
	}

	// Good source's skill should still be installed despite the bad source.
	if !m.HasSkill("good-source", "skill-from-a") {
		t.Error("skill-from-a (good-source) should survive sync despite bad-source failure")
	}

	// Bad source should have no skills in the manifest.
	badSkills := m.SkillsBySource("bad-source")
	if len(badSkills) != 0 {
		t.Errorf("bad-source should have no skills after failed clone, got %d", len(badSkills))
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

	// Update source to point to new repo (multi-source format).
	updatedSource := config.SourceConfig{
		Alias:  "default",
		URL:    "file://" + newRepoDir,
		Branch: "main",
	}
	if err := config.CreateSource("default", updatedSource); err != nil {
		t.Fatalf("updating default source: %v", err)
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
	if m.HasSkill("default", "skill-b") {
		t.Error("manifest should not have skill-b after prune")
	}
	if !m.HasSkill("default", "skill-a") {
		t.Error("manifest should have skill-a after prune")
	}
}

func TestSyncCmd_SameNameDiffSource(t *testing.T) {
	// Create two repos, each with a skill named "foo".
	repoA := t.TempDir()
	createLocalRepo(t, repoA, "main", []string{"foo"})

	repoB := t.TempDir()
	createLocalRepo(t, repoB, "main", []string{"foo"})

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Source A and Source B.
	if err := config.CreateSource("source-a", config.SourceConfig{
		Alias:  "source-a",
		URL:    "file://" + repoA,
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating source-a: %v", err)
	}
	if err := config.CreateSource("source-b", config.SourceConfig{
		Alias:  "source-b",
		URL:    "file://" + repoB,
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating source-b: %v", err)
	}

	cfg := &config.Config{
		TargetDir: "~/.config/opencode/skills",
		Sources:   []string{"source-a", "source-b"},
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// Install from both sources.
	root := NewRootCmd()
	root.SetArgs([]string{"install", "--all", "--source", "source-a"})
	if err := root.Execute(); err != nil {
		t.Fatalf("install --all --source source-a failed: %v", err)
	}

	root = NewRootCmd()
	root.SetArgs([]string{"install", "--all", "--source", "source-b"})
	if err := root.Execute(); err != nil {
		t.Fatalf("install --all --source source-b failed: %v", err)
	}

	// Verify both foo skills coexist in the manifest with different sources.
	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")
	m, err := manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("loading manifest: %v", err)
	}

	if !m.HasSkill("source-a", "foo") {
		t.Error("foo from source-a should be in manifest")
	}
	if !m.HasSkill("source-b", "foo") {
		t.Error("foo from source-b should be in manifest")
	}

	// Verify SkillsBySource shows correct counts.
	aSkills := m.SkillsBySource("source-a")
	bSkills := m.SkillsBySource("source-b")

	if len(aSkills) != 1 {
		t.Errorf("source-a should have 1 skill, got %d", len(aSkills))
	}
	if len(bSkills) != 1 {
		t.Errorf("source-b should have 1 skill, got %d", len(bSkills))
	}

	// Verify display: list should show both with alias.
	root = NewRootCmd()
	root.SetArgs([]string{"list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	// Remove one, verify the other remains.
	root = NewRootCmd()
	root.SetArgs([]string{"remove", "source-a:foo"})
	if err := root.Execute(); err != nil {
		t.Fatalf("remove source-a:foo failed: %v", err)
	}

	m, err = manifest.Load(manifestPath)
	if err != nil {
		t.Fatalf("reloading manifest after remove: %v", err)
	}

	if m.HasSkill("source-a", "foo") {
		t.Error("foo from source-a should be removed")
	}
	if !m.HasSkill("source-b", "foo") {
		t.Error("foo from source-b should still exist")
	}
}
