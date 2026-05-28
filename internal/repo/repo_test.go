package repo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// createSourceRepo creates a go-git repository at dir with skill directories
// and commits them. Returns the repo reference.
func createSourceRepo(t *testing.T, dir string, initialBranch string) {
	t.Helper()

	opts := &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.NewBranchReferenceName(initialBranch),
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

	// Create skill directories with files
	skillFiles := map[string]string{
		"my-skill/README.md":    "# My Skill\n",
		"my-skill/main.go":      "package myskill\n",
		"other-skill/main.go":   "package main\n",
		"other-skill/util.go":   "package main\n\nfunc Util() {}\n",
		".hidden-skill/file.go": "should not appear\n",
	}

	for fpath, content := range skillFiles {
		full := filepath.Join(dir, fpath)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", fpath, err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", fpath, err)
		}
	}

	// Add all files
	if _, err := w.Add("my-skill/README.md"); err != nil {
		t.Fatalf("Add my-skill/README.md: %v", err)
	}
	if _, err := w.Add("my-skill/main.go"); err != nil {
		t.Fatalf("Add my-skill/main.go: %v", err)
	}
	if _, err := w.Add("other-skill/main.go"); err != nil {
		t.Fatalf("Add other-skill/main.go: %v", err)
	}
	if _, err := w.Add("other-skill/util.go"); err != nil {
		t.Fatalf("Add other-skill/util.go: %v", err)
	}
	// Do NOT add .hidden-skill — it should not appear in ListSkillDirs

	commitHash, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com"},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	_ = commitHash
}

func TestCloneShallowAndListSkillDirs(t *testing.T) {
	srcDir := t.TempDir()
	createSourceRepo(t, srcDir, "main")

	destDir := t.TempDir()
	client := &Client{
		URL:    srcDir,
		Branch: "main",
	}

	ctx := context.Background()
	if err := client.CloneShallow(ctx, destDir); err != nil {
		t.Fatalf("CloneShallow failed: %v", err)
	}

	// Verify the clone exists
	if _, err := os.Stat(filepath.Join(destDir, ".git")); os.IsNotExist(err) {
		t.Fatal("CloneShallow did not create .git directory")
	}

	skills, err := client.ListSkillDirs(ctx, destDir)
	if err != nil {
		t.Fatalf("ListSkillDirs failed: %v", err)
	}

	// We expect "my-skill" and "other-skill" but NOT ".hidden-skill" or any files
	if len(skills) != 2 {
		t.Fatalf("ListSkillDirs returned %d skills, want 2: %+v", len(skills), skills)
	}

	// Check names
	names := make(map[string]string)
	for _, s := range skills {
		names[s.Name] = s.TreeSHA
	}

	if _, ok := names["my-skill"]; !ok {
		t.Error("Missing 'my-skill' in listed skills")
	}
	if _, ok := names["other-skill"]; !ok {
		t.Error("Missing 'other-skill' in listed skills")
	}
	if _, ok := names[".hidden-skill"]; ok {
		t.Error("'.hidden-skill' should not be listed")
	}

	// Tree SHAs should be non-empty
	for _, s := range skills {
		if s.TreeSHA == "" {
			t.Errorf("Skill %q has empty TreeSHA", s.Name)
		}
	}
}

func TestListSkillDirsTreeSHAs(t *testing.T) {
	srcDir := t.TempDir()

	// Create a repo with just one skill
	opts := &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.NewBranchReferenceName("main"),
		},
	}
	r, err := git.PlainInitWithOptions(srcDir, opts)
	if err != nil {
		t.Fatalf("PlainInitWithOptions: %v", err)
	}

	w, err := r.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	os.MkdirAll(filepath.Join(srcDir, "test-skill"), 0755)
	os.WriteFile(filepath.Join(srcDir, "test-skill", "a.txt"), []byte("content a"), 0644)
	w.Add("test-skill/a.txt")

	firstCommit, err := w.Commit("Add test-skill", &git.CommitOptions{
		Author: &object.Signature{Name: "Test"},
	})
	if err != nil {
		t.Fatalf("First commit: %v", err)
	}

	// Add another file to the skill in a second commit
	os.WriteFile(filepath.Join(srcDir, "test-skill", "b.txt"), []byte("content b"), 0644)
	w.Add("test-skill/b.txt")

	secondCommit, err := w.Commit("Add b.txt", &git.CommitOptions{
		Author: &object.Signature{Name: "Test"},
	})
	if err != nil {
		t.Fatalf("Second commit: %v", err)
	}
	_ = firstCommit
	_ = secondCommit

	// Clone and check
	destDir := t.TempDir()
	client := &Client{URL: srcDir, Branch: "main"}

	if err := client.CloneShallow(context.Background(), destDir); err != nil {
		t.Fatalf("CloneShallow: %v", err)
	}

	skills, err := client.ListSkillDirs(context.Background(), destDir)
	if err != nil {
		t.Fatalf("ListSkillDirs: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	// The tree SHA should be deterministic and non-empty
	sha := skills[0].TreeSHA
	if sha == "" {
		t.Fatal("TreeSHA should not be empty")
	}
	// SHA format: 40 hex chars
	if len(sha) != 40 {
		t.Errorf("TreeSHA length = %d, want 40", len(sha))
	}
}

func TestComputeLocalTreeSHA(t *testing.T) {
	dir := t.TempDir()

	// Create skill-a with nested file structure
	os.MkdirAll(filepath.Join(dir, "skill-a", "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "skill-a", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "skill-a", "subdir", "util.go"), []byte("package subdir"), 0644)

	// Compute SHA for skill-a
	sha1, err := ComputeLocalTreeSHA(filepath.Join(dir, "skill-a"))
	if err != nil {
		t.Fatalf("ComputeLocalTreeSHA failed: %v", err)
	}
	if sha1 == "" {
		t.Fatal("SHA is empty")
	}

	// Should be deterministic
	sha2, err := ComputeLocalTreeSHA(filepath.Join(dir, "skill-a"))
	if err != nil {
		t.Fatalf("ComputeLocalTreeSHA (2nd call) failed: %v", err)
	}
	if sha1 != sha2 {
		t.Error("SHA should be deterministic (same content, same hash)")
	}
}

func TestComputeLocalTreeSHAChangesOnModification(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "file.go"), []byte("original"), 0644)

	shaOriginal, err := ComputeLocalTreeSHA(skillDir)
	if err != nil {
		t.Fatalf("ComputeLocalTreeSHA (original) failed: %v", err)
	}

	// Modify the file content
	os.WriteFile(filepath.Join(skillDir, "file.go"), []byte("modified content"), 0644)

	shaModified, err := ComputeLocalTreeSHA(skillDir)
	if err != nil {
		t.Fatalf("ComputeLocalTreeSHA (modified) failed: %v", err)
	}

	if shaOriginal == shaModified {
		t.Error("SHA should change when file content changes")
	}
}

func TestComputeLocalTreeSHANotADirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	os.WriteFile(filePath, []byte("hello"), 0644)

	_, err := ComputeLocalTreeSHA(filePath)
	if err == nil {
		t.Fatal("expected error for non-directory path, got nil")
	}
}

func TestComputeLocalTreeSHASkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skill")
	os.MkdirAll(skillDir, 0755)

	// Add a visible file
	os.WriteFile(filepath.Join(skillDir, "visible.go"), []byte("visible"), 0644)

	shaWithVisible, err := ComputeLocalTreeSHA(skillDir)
	if err != nil {
		t.Fatalf("ComputeLocalTreeSHA (visible) failed: %v", err)
	}

	// Add a hidden file — should not affect hash
	os.WriteFile(filepath.Join(skillDir, ".hidden"), []byte("secret"), 0644)

	shaWithHidden, err := ComputeLocalTreeSHA(skillDir)
	if err != nil {
		t.Fatalf("ComputeLocalTreeSHA (hidden) failed: %v", err)
	}

	if shaWithVisible != shaWithHidden {
		t.Error("Hidden files should be excluded from hash computation")
	}
}

func TestCloneShallowWithDefaultBranch(t *testing.T) {
	// Test that cloning works even when the source repo uses "master" as default
	srcDir := t.TempDir()
	createSourceRepo(t, srcDir, "master")

	destDir := t.TempDir()
	client := &Client{
		URL:    srcDir,
		Branch: "master",
	}

	ctx := context.Background()
	if err := client.CloneShallow(ctx, destDir); err != nil {
		t.Fatalf("CloneShallow with master branch failed: %v", err)
	}

	skills, err := client.ListSkillDirs(ctx, destDir)
	if err != nil {
		t.Fatalf("ListSkillDirs failed: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}
