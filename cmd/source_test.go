package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vekzz-dev/slap-skills/internal/config"
)

// setupSourceEnv creates a temporary home directory with the slap config
// in a migrated (multi-source) state and returns the home path.
func setupSourceEnv(t *testing.T) (homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	t.Setenv("HOME", homeDir)
	return homeDir
}

// captureStdout runs fn while capturing anything written to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	w.Close()
	out := <-done
	os.Stdout = old
	return out
}

// createSourceFile writes a source YAML file for the given alias inside the
// sources directory under the given home directory.
func createSourceFile(t *testing.T, homeDir, alias, url, branch string) {
	t.Helper()
	sourcesDir := filepath.Join(homeDir, ".config", "slap", "sources")
	if err := os.MkdirAll(sourcesDir, 0755); err != nil {
		t.Fatalf("MkdirAll sources: %v", err)
	}
	content := "alias: " + alias + "\nurl: " + url + "\nbranch: " + branch + "\n"
	path := filepath.Join(sourcesDir, alias+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// createSlapConfig writes a minimal multi-source slap config.
func createSlapConfig(t *testing.T, homeDir, targetDir string) {
	t.Helper()
	cfgDir := filepath.Join(homeDir, ".config", "slap")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("MkdirAll config: %v", err)
	}
	content := "target_dir: " + targetDir + "\nsources:\n  - default\n"
	path := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func TestSourceListCmd_Empty(t *testing.T) {
	setupSourceEnv(t)

	root := NewRootCmd()
	root.SetArgs([]string{"source", "list"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("source list on empty config: %v", err)
		}
	})

	if !strings.Contains(out, "No sources configured") {
		t.Errorf("expected 'No sources configured', got:\n%s", out)
	}
}

func TestSourceListCmd_WithSources(t *testing.T) {
	homeDir := setupSourceEnv(t)
	createSourceFile(t, homeDir, "work", "https://github.com/work/skills", "main")
	createSourceFile(t, homeDir, "community", "https://github.com/community/skills", "develop")

	root := NewRootCmd()
	root.SetArgs([]string{"source", "list"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("source list with sources: %v", err)
		}
	})

	for _, want := range []string{"work", "community"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestSourceListCmd_AfterMigration(t *testing.T) {
	homeDir := setupSourceEnv(t)

	// Create an old-style config (repo_url set, no sources dir).
	cfgDir := filepath.Join(homeDir, ".config", "slap")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	oldContent := "repo_url: https://github.com/example/old-repo\nbranch: main\ntarget_dir: ~/.config/opencode/skills\n"
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(oldContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"source", "list"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("source list after migration: %v", err)
		}
	})

	// After migration, a "default" source should exist.
	if !strings.Contains(out, "default") {
		t.Errorf("expected output to contain 'default' after migration, got:\n%s", out)
	}

	// Verify sources dir and default source file were migrated.
	sourcesDir := filepath.Join(homeDir, ".config", "slap", "sources")
	if _, err := os.Stat(sourcesDir); os.IsNotExist(err) {
		t.Error("expected sources directory to exist after migration")
	}
	if _, err := os.Stat(filepath.Join(sourcesDir, "default.yaml")); os.IsNotExist(err) {
		t.Error("expected default.yaml to exist after migration")
	}
}

func TestSourceRemoveCmd_NotFound(t *testing.T) {
	homeDir := setupSourceEnv(t)
	createSlapConfig(t, homeDir, "~/.config/opencode/skills")

	root := NewRootCmd()
	root.SetArgs([]string{"source", "remove", "nonexistent"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent source, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got: %v", err)
	}
}

func TestSourceRemoveCmd_NoSkills(t *testing.T) {
	homeDir := setupSourceEnv(t)
	createSlapConfig(t, homeDir, "~/.config/opencode/skills")
	createSourceFile(t, homeDir, "test-source", "https://github.com/test/skills", "main")

	root := NewRootCmd()
	root.SetArgs([]string{"source", "remove", "test-source"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("source remove with no skills: %v", err)
		}
	})

	if !strings.Contains(out, "removed") {
		t.Errorf("expected 'removed' in output, got:\n%s", out)
	}

	// Verify source file was deleted.
	expanded := expandPath(config.SlapDir)
	if _, err := os.Stat(filepath.Join(expanded, "sources", "test-source.yaml")); !os.IsNotExist(err) {
		t.Error("expected source file to be deleted")
	}
}
