package manifest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vekzz-dev/slap-skills/internal/repo"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	m := &Manifest{
		Version:      1,
		SourceRepo:   "https://github.com/user/skills.git",
		SourceBranch: "main",
		LastSync:     now,
		Skills: map[string]SkillEntry{
			"my-skill": {
				SHA:          "abc123def456",
				InstalledAt:  now,
				LastSyncedAt: now,
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	if err := m.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != m.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, m.Version)
	}
	if loaded.SourceRepo != m.SourceRepo {
		t.Errorf("SourceRepo = %q, want %q", loaded.SourceRepo, m.SourceRepo)
	}
	if loaded.SourceBranch != m.SourceBranch {
		t.Errorf("SourceBranch = %q, want %q", loaded.SourceBranch, m.SourceBranch)
	}

	entry, ok := loaded.Skills["my-skill"]
	if !ok {
		t.Fatal("Skills['my-skill']: missing")
	}
	if entry.SHA != "abc123def456" {
		t.Errorf("SHA = %q, want %q", entry.SHA, "abc123def456")
	}
	if !entry.InstalledAt.Equal(now) {
		t.Errorf("InstalledAt = %v, want %v", entry.InstalledAt, now)
	}
}

func TestAtomicSavePartialWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Create initial manifest
	m1 := &Manifest{
		Version: 1,
		Skills: map[string]SkillEntry{
			"skill-a": {SHA: "aaa", InstalledAt: time.Now(), LastSyncedAt: time.Now()},
		},
	}
	if err := m1.Save(path); err != nil {
		t.Fatalf("initial Save failed: %v", err)
	}

	// Simulate a partial/corrupt write by writing garbage directly
	if err := os.WriteFile(path, []byte("{partial garbage"), 0644); err != nil {
		t.Fatalf("WriteFile (partial) failed: %v", err)
	}

	// Load should detect corruption, back up, and return empty
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after corruption failed: %v", err)
	}

	if len(loaded.Skills) != 0 {
		t.Errorf("expected empty Skills after corruption, got %d entries", len(loaded.Skills))
	}

	// Backup file should exist
	if _, err := os.Stat(path + ".bak"); os.IsNotExist(err) {
		t.Fatal("expected .bak file for corrupt manifest, but none found")
	}
}

func TestLoadCorruptJSONCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	content := []byte(`{"version": 1, "skills": "this is not valid at this position`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load corrupt JSON failed: %v", err)
	}

	if m == nil {
		t.Fatal("Load returned nil manifest")
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected empty Skills, got %d entries", len(m.Skills))
	}

	// Backup should exist with original content
	backupContent, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backupContent) != string(content) {
		t.Errorf("backup content mismatch:\ngot  %q\nwant %q", string(backupContent), string(content))
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	m, err := Load("/nonexistent/manifest.json")
	if err != nil {
		t.Fatalf("Load missing file failed: %v", err)
	}
	if m == nil {
		t.Fatal("Load returned nil for missing file")
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected empty Skills, got %d entries", len(m.Skills))
	}
}

func TestUpsertSkill(t *testing.T) {
	m := emptyManifest()

	// Add new skill
	m.UpsertSkill("", "foo", "abc123")
	if !m.HasSkill("", "foo") {
		t.Fatal("HasSkill('foo') = false after UpsertSkill")
	}
	if m.Skills["foo"].SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", m.Skills["foo"].SHA, "abc123")
	}
	if m.Skills["foo"].InstalledAt.IsZero() {
		t.Error("InstalledAt should be set")
	}

	// Update existing skill
	installedAt := m.Skills["foo"].InstalledAt
	m.UpsertSkill("", "foo", "def456")
	if m.Skills["foo"].SHA != "def456" {
		t.Errorf("SHA after update = %q, want %q", m.Skills["foo"].SHA, "def456")
	}
	if !m.Skills["foo"].InstalledAt.Equal(installedAt) {
		t.Error("InstalledAt should not change on update")
	}
}

func TestRemoveSkill(t *testing.T) {
	m := emptyManifest()
	m.UpsertSkill("", "foo", "abc")
	m.RemoveSkill("", "foo")
	if m.HasSkill("", "foo") {
		t.Fatal("HasSkill('foo') = true after RemoveSkill")
	}
}

func TestHasSkill(t *testing.T) {
	m := emptyManifest()
	if m.HasSkill("", "nonexistent") {
		t.Fatal("HasSkill('nonexistent') = true on empty manifest")
	}

	m.UpsertSkill("", "bar", "xyz")
	if !m.HasSkill("", "bar") {
		t.Fatal("HasSkill('bar') = false after UpsertSkill")
	}
}

func TestRebuildFromDiskMixedState(t *testing.T) {
	dir := t.TempDir()

	// Create target dir with some matching and some non-matching folders
	os.MkdirAll(filepath.Join(dir, "skill-a", "nested"), 0755)
	os.WriteFile(filepath.Join(dir, "skill-a", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "skill-a", "nested", "helper.go"), []byte("package nested"), 0644)

	os.MkdirAll(filepath.Join(dir, "skill-b"), 0755)
	os.WriteFile(filepath.Join(dir, "skill-b", "README.md"), []byte("# B"), 0644)

	os.MkdirAll(filepath.Join(dir, "non-managed"), 0755) // not in repo skills
	os.WriteFile(filepath.Join(dir, "non-managed", "file.go"), []byte("package nm"), 0644)

	repoSkills := []repo.SkillDir{
		{Name: "skill-a", TreeSHA: "should-be-overwritten-by-local-sha"},
		{Name: "skill-b", TreeSHA: "should-be-overwritten-by-local-sha"},
	}

	m, err := RebuildFromDisk(dir, repoSkills)
	if err != nil {
		t.Fatalf("RebuildFromDisk failed: %v", err)
	}

	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}

	// Should contain both repo-matching skills
	if !m.HasSkill("", "skill-a") {
		t.Error("Missing skill-a in rebuilt manifest")
	}
	if !m.HasSkill("", "skill-b") {
		t.Error("Missing skill-b in rebuilt manifest")
	}

	// Should NOT contain non-managed folder
	if m.HasSkill("", "non-managed") {
		t.Error("non-managed folder should not be in rebuilt manifest")
	}

	// Local SHAs should be computed (non-empty)
	if m.Skills["skill-a"].SHA == "" {
		t.Error("skill-a SHA should not be empty")
	}
	if m.Skills["skill-b"].SHA == "" {
		t.Error("skill-b SHA should not be empty")
	}

	// Same content should produce same SHA
	m2, err := RebuildFromDisk(dir, repoSkills)
	if err != nil {
		t.Fatalf("RebuildFromDisk (2nd call) failed: %v", err)
	}
	if m.Skills["skill-a"].SHA != m2.Skills["skill-a"].SHA {
		t.Error("RebuildFromDisk should be deterministic: skill-a SHA changed")
	}

	// Non-existent target dir returns empty manifest
	m3, err := RebuildFromDisk("/nonexistent-target", repoSkills)
	if err != nil {
		t.Fatalf("RebuildFromDisk with missing dir failed: %v", err)
	}
	if len(m3.Skills) != 0 {
		t.Errorf("expected empty Skills for missing dir, got %d", len(m3.Skills))
	}
}

func TestRepairCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	content := []byte(`garbage content`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// RepairCorrupt moves the file to .bak
	if err := RepairCorrupt(path); err != nil {
		t.Fatalf("RepairCorrupt failed: %v", err)
	}

	// Original should be removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("RepairCorrupt did not remove original file")
	}

	// Backup should exist
	bakContent, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(bakContent) != string(content) {
		t.Errorf("backup content mismatch:\ngot  %q\nwant %q", string(bakContent), string(content))
	}
}

func TestRepairCorruptNoopOnMissing(t *testing.T) {
	if err := RepairCorrupt("/nonexistent/manifest.json"); err != nil {
		t.Fatalf("RepairCorrupt on missing file should be no-op: %v", err)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Write empty file
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load empty file failed: %v", err)
	}
	if m == nil {
		t.Fatal("Load returned nil for empty file")
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected empty Skills for empty file, got %d", len(m.Skills))
	}
}

func TestVersionDefaultsToOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Load explicitly saved file without Version set
	m := &Manifest{
		Skills: map[string]SkillEntry{
			"test": {SHA: "abc", InstalledAt: time.Now(), LastSyncedAt: time.Now()},
		},
	}
	if err := m.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Version != 1 {
		t.Errorf("Version = %d, want 1", loaded.Version)
	}
}

func TestManifestKey(t *testing.T) {
	tests := []struct {
		source string
		name   string
		want   string
	}{
		{"", "foo", "foo"},
		{"default", "foo", "default:foo"},
		{"work", "my-skill", "work:my-skill"},
		{"", "default:foo", "default:foo"},
	}
	for _, tt := range tests {
		got := manifestKey(tt.source, tt.name)
		if got != tt.want {
			t.Errorf("manifestKey(%q, %q) = %q, want %q", tt.source, tt.name, got, tt.want)
		}
	}
}

func TestHasSkillWithSource(t *testing.T) {
	m := emptyManifest()
	m.UpsertSkill("default", "foo", "abc123")

	if !m.HasSkill("default", "foo") {
		t.Error("HasSkill('default', 'foo') should be true")
	}
	if m.HasSkill("default", "nonexistent") {
		t.Error("HasSkill('default', 'nonexistent') should be false")
	}
	if m.HasSkill("other", "foo") {
		t.Error("HasSkill('other', 'foo') should be false (wrong source)")
	}
	if m.HasSkill("", "foo") {
		t.Error("HasSkill('', 'foo') should be false (empty source, key = 'default:foo')")
	}
}

func TestSkillsBySource(t *testing.T) {
	m := emptyManifest()
	m.UpsertSkill("default", "skill-a", "sha1")
	m.UpsertSkill("default", "skill-b", "sha2")
	m.UpsertSkill("work", "skill-c", "sha3")

	defaultSkills := m.SkillsBySource("default")
	if len(defaultSkills) != 2 {
		t.Errorf("SkillsBySource('default') len = %d, want 2", len(defaultSkills))
	}
	if defaultSkills["skill-a"].SHA != "sha1" {
		t.Errorf("skill-a SHA = %q, want %q", defaultSkills["skill-a"].SHA, "sha1")
	}
	if defaultSkills["skill-b"].SHA != "sha2" {
		t.Errorf("skill-b SHA = %q, want %q", defaultSkills["skill-b"].SHA, "sha2")
	}

	workSkills := m.SkillsBySource("work")
	if len(workSkills) != 1 {
		t.Errorf("SkillsBySource('work') len = %d, want 1", len(workSkills))
	}

	emptySkills := m.SkillsBySource("nonexistent")
	if len(emptySkills) != 0 {
		t.Errorf("SkillsBySource('nonexistent') len = %d, want 0", len(emptySkills))
	}
}

func TestRemoveBySource(t *testing.T) {
	m := emptyManifest()
	m.UpsertSkill("default", "skill-a", "sha1")
	m.UpsertSkill("default", "skill-b", "sha2")
	m.UpsertSkill("work", "skill-c", "sha3")

	removed := m.RemoveBySource("default")
	if removed != 2 {
		t.Errorf("RemoveBySource('default') returned %d, want 2", removed)
	}
	if m.HasSkill("default", "skill-a") {
		t.Error("skill-a should be removed")
	}
	if m.HasSkill("default", "skill-b") {
		t.Error("skill-b should be removed")
	}
	if !m.HasSkill("work", "skill-c") {
		t.Error("skill-c should still exist (different source)")
	}

	removed = m.RemoveBySource("work")
	if removed != 1 {
		t.Errorf("RemoveBySource('work') returned %d, want 1", removed)
	}
	if len(m.Skills) != 0 {
		t.Errorf("expected empty manifest, got %d entries", len(m.Skills))
	}
}

func TestRemoveSkillWithSource(t *testing.T) {
	m := emptyManifest()
	m.UpsertSkill("default", "foo", "abc")
	m.UpsertSkill("work", "foo", "def") // same name, different source

	m.RemoveSkill("default", "foo")
	if m.HasSkill("default", "foo") {
		t.Error("'default:foo' should be removed")
	}
	if !m.HasSkill("work", "foo") {
		t.Error("'work:foo' should still exist")
	}

	m.RemoveSkill("work", "foo")
	if m.HasSkill("work", "foo") {
		t.Error("'work:foo' should be removed")
	}
}

func TestUpsertSkillWithSource(t *testing.T) {
	m := emptyManifest()

	// Add new skill with source
	m.UpsertSkill("default", "foo", "abc123")
	if m.Skills["default:foo"].SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", m.Skills["default:foo"].SHA, "abc123")
	}
	if m.Skills["default:foo"].Source != "default" {
		t.Errorf("Source = %q, want %q", m.Skills["default:foo"].Source, "default")
	}
	if m.Skills["default:foo"].InstalledAt.IsZero() {
		t.Error("InstalledAt should be set")
	}

	// Update existing skill — InstalledAt should not change
	installedAt := m.Skills["default:foo"].InstalledAt
	m.UpsertSkill("default", "foo", "def456")
	if m.Skills["default:foo"].SHA != "def456" {
		t.Errorf("SHA after update = %q, want %q", m.Skills["default:foo"].SHA, "def456")
	}
	if !m.Skills["default:foo"].InstalledAt.Equal(installedAt) {
		t.Error("InstalledAt should not change on update")
	}

	// Add skill with empty source (backward compat)
	m.UpsertSkill("", "bar", "xyz")
	if m.Skills["bar"].SHA != "xyz" {
		t.Errorf("backward compat: SHA = %q, want %q", m.Skills["bar"].SHA, "xyz")
	}
}

func TestSkillsBySourceBackwardCompat(t *testing.T) {
	m := emptyManifest()
	now := time.Now()

	// Manually set a legacy skill (key = name, Source field not set)
	m.Skills["legacy-skill"] = SkillEntry{SHA: "old-sha", InstalledAt: now, LastSyncedAt: now}

	// Add a new-style skill
	m.UpsertSkill("default", "new-skill", "new-sha")

	// SkillsBySource with empty source should include legacy
	emptySrc := m.SkillsBySource("")
	if len(emptySrc) != 1 {
		t.Errorf("SkillsBySource('') len = %d, want 1 (legacy)", len(emptySrc))
	}

	// SkillsBySource with "default" should include new-skill
	defaultSrc := m.SkillsBySource("default")
	if len(defaultSrc) != 1 {
		t.Errorf("SkillsBySource('default') len = %d, want 1", len(defaultSrc))
	}
	if defaultSrc["new-skill"].SHA != "new-sha" {
		t.Errorf("new-skill SHA = %q, want %q", defaultSrc["new-skill"].SHA, "new-sha")
	}
}
