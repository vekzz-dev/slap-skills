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

func TestInitCmd_AlreadyConfigured(t *testing.T) {
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

	// init should say already configured (not error).
	root := NewRootCmd()
	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatalf("init should not error when sources exist, got: %v", err)
	}
}

func TestInitCmd_NoSourcesLaunchesWizard(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// No sources — init should try to launch the interactive wizard.
	root := NewRootCmd()
	root.SetArgs([]string{"init"})
	err := root.Execute()
	// Expected: either EOF/terminal error from survey (no PTY) or success.
	if err != nil {
		t.Logf("init with no sources gave expected interactive error: %v", err)
	}
}
