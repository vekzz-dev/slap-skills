package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vekzz-dev/skills-manager/internal/manifest"
)

// writeManifest creates and saves a manifest at the slap manifest path
// inside the given home directory.
func writeManifest(t *testing.T, homeDir string, skills map[string]string) {
	t.Helper()

	m := &manifest.Manifest{
		Version:      1,
		SourceRepo:   "https://example.com/repo.git",
		SourceBranch: "main",
		LastSync:     time.Now(),
		Skills:       make(map[string]manifest.SkillEntry, len(skills)),
	}
	now := time.Now()
	for name, sha := range skills {
		m.Skills[name] = manifest.SkillEntry{
			SHA:          sha,
			InstalledAt:  now,
			LastSyncedAt: now,
		}
	}

	manifestPath := filepath.Join(homeDir, ".config", "slap", "manifest.json")
	if err := m.Save(manifestPath); err != nil {
		t.Fatalf("saving test manifest: %v", err)
	}
}

func TestListCmd_TableOutput(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeManifest(t, homeDir, map[string]string{
		"test-skill": "abcdef1234567890",
	})

	// Capture stdout.
	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "test-skill") {
		t.Errorf("output should contain skill name, got:\n%s", out)
	}
	if !strings.Contains(out, "abcdef1") {
		t.Errorf("output should contain shortened SHA, got:\n%s", out)
	}
	if !strings.Contains(out, "Skill Name") {
		t.Errorf("output should contain table header, got:\n%s", out)
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeManifest(t, homeDir, map[string]string{
		"json-skill": "deadbeefcafe",
	})

	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("list --json failed: %v", err)
	}

	// Verify it's valid JSON.
	var parsed manifest.Manifest
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if !parsed.HasSkill("json-skill") {
		t.Error("JSON output missing json-skill")
	}
}

func TestListCmd_EmptyManifest(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	root := NewRootCmd()
	root.SetArgs([]string{"list"})
	err := root.Execute()
	if err == nil {
		t.Fatal("list with no manifest should have failed, got nil")
	}
	if !strings.Contains(err.Error(), "no skills installed") {
		t.Errorf("error should mention 'no skills installed', got: %v", err)
	}
}
