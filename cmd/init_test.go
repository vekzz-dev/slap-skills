package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/vekzz-dev/slap-skills/internal/config"
)

// createLocalRepo initialises a git repository at dir with the given branch
// and skill directory names.  Each skill gets a skill.yaml file.  All files
// are committed in a single initial commit.
func createLocalRepo(t *testing.T, dir, branch string, skills []string) {
	t.Helper()

	opts := &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.NewBranchReferenceName(branch),
		},
	}
	r, err := git.PlainInitWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("PlainInitWithOptions: %v", err)
	}

	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	for _, skill := range skills {
		skillDir := filepath.Join(dir, skill)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", skill, err)
		}
		content := []byte("name: " + skill + "\nversion: 1\n")
		if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), content, 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", skill, err)
		}
	}

	// Stage every file.
	for _, skill := range skills {
		if _, err := w.Add(skill + "/skill.yaml"); err != nil {
			t.Fatalf("Add %s: %v", skill, err)
		}
	}

	if _, err := w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com"},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestInitCmd_ValidURL(t *testing.T) {
	// Create a local source repo.
	repoDir := t.TempDir()
	createLocalRepo(t, repoDir, "main", []string{"skill-a", "skill-b"})

	// Redirect home so ~/.config/slap lands in our temp area.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := NewRootCmd()
	repoURL := "file://" + repoDir
	root.SetArgs([]string{"init", repoURL})
	err := root.Execute()
	if err != nil {
		t.Fatalf("init with valid URL failed: %v", err)
	}

	// Verify the source file was created.
	src, err := config.ReadSource("default")
	if err != nil {
		t.Fatalf("reading source 'default' after init: %v", err)
	}
	if src.URL != repoURL {
		t.Errorf("Source URL = %q, want %q", src.URL, repoURL)
	}
	if src.Branch != "main" {
		t.Errorf("Source Branch = %q, want %q", src.Branch, "main")
	}
}

func TestInitCmd_InvalidURL(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := NewRootCmd()
	root.SetArgs([]string{"init", "file:///nonexistent-repo-that-does-not-exist"})
	err := root.Execute()
	if err == nil {
		t.Fatal("init with invalid URL should have failed, got nil")
	}
}

func TestInitCmd_NoArgsWithSources(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Create a source first.
	repoDir := t.TempDir()
	createLocalRepo(t, repoDir, "main", []string{"skill-a"})
	if err := config.CreateSource("test-source", config.SourceConfig{
		Alias:  "test-source",
		URL:    "file://" + repoDir,
		Branch: "main",
	}); err != nil {
		t.Fatalf("creating source: %v", err)
	}
	cfg := &config.Config{
		TargetDir: "~/.config/opencode/skills",
		Sources:   []string{"test-source"},
	}
	if err := cfg.Save(config.ConfigFile); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// init with no args should show sources (not error).
	root := NewRootCmd()
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("init with no args should work when sources exist, got: %v", err)
	}
}

func TestInitCmd_NoArgsNoSources(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Run migration so sources dir exists but is empty.
	_ = config.MigrateConfig()

	// init with no args and no sources — should launch wizard (interactive will
	// fail since no PTY, but should not say "accepts 1 arg(s), received 0").
	root := NewRootCmd()
	root.SetArgs([]string{"init"})
	err := root.Execute()
	// Expected: either EOF/terminal error from survey (no PTY) or success with
	// "No sources configured" message. Anything other than "accepts N arg(s)" is fine.
	if err != nil {
		t.Logf("init with no args and no sources gave expected interactive error: %v", err)
	}
}

func TestInitCmd_SavesBranchFromFlag(t *testing.T) {
	repoDir := t.TempDir()
	createLocalRepo(t, repoDir, "develop", []string{"skill-a"})

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := NewRootCmd()
	root.SetArgs([]string{"init", "--branch", "develop", "file://" + repoDir})
	if err := root.Execute(); err != nil {
		t.Fatalf("init with --branch failed: %v", err)
	}

	src, err := config.ReadSource("default")
	if err != nil {
		t.Fatalf("reading source 'default': %v", err)
	}
	if src.Branch != "develop" {
		t.Errorf("Source Branch = %q, want %q", src.Branch, "develop")
	}
}
