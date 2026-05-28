package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/vekzz-dev/skills-manager/internal/config"
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

	// Verify the config file was created and contains the right values.
	cfg, err := config.Load(config.ConfigFile)
	if err != nil {
		t.Fatalf("loading config after init: %v", err)
	}
	if cfg.RepoURL != repoURL {
		t.Errorf("RepoURL = %q, want %q", cfg.RepoURL, repoURL)
	}
	if cfg.Branch != "main" {
		t.Errorf("Branch = %q, want %q", cfg.Branch, "main")
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

func TestInitCmd_MissingArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"init"})
	err := root.Execute()
	if err == nil {
		t.Fatal("init without args should have failed, got nil")
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

	cfg, err := config.Load(config.ConfigFile)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Branch != "develop" {
		t.Errorf("Branch = %q, want %q", cfg.Branch, "develop")
	}
}
