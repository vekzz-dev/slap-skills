// Package manifest manages the slap manifest file that tracks installed skills.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vekzz-dev/slap-skills/internal/repo"
)

// SkillEntry represents a single installed skill in the manifest.
type SkillEntry struct {
	SHA          string    `json:"sha"`
	InstalledAt  time.Time `json:"installed_at"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

// Manifest tracks the state of all installed skills from a source repo.
type Manifest struct {
	Version      int                   `json:"version"`
	SourceRepo   string                `json:"source_repo"`
	SourceBranch string                `json:"source_branch"`
	LastSync     time.Time             `json:"last_sync"`
	Skills       map[string]SkillEntry `json:"skills"`
}

// Load reads a manifest from the given file path.
//
// Behaviour:
//   - If the file does not exist, returns an empty, initialised Manifest.
//   - If the file contains invalid JSON, backs it up to <path>.bak and returns
//     an empty Manifest.
//   - Any other read error is returned to the caller.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyManifest(), nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return emptyManifest(), nil
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Back up corrupt file before proceeding
		backupPath := path + ".bak"
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr != nil {
			return nil, fmt.Errorf(
				"manifest corrupt and backup failed at %s: %w (original error: %v)",
				backupPath, writeErr, err,
			)
		}
		return emptyManifest(), nil
	}

	if m.Skills == nil {
		m.Skills = make(map[string]SkillEntry)
	}
	if m.Version == 0 {
		m.Version = 1
	}

	return &m, nil
}

// Save writes the manifest atomically to the given path.
// It creates the parent directory if it does not exist.
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	// Atomic write: temp file + rename
	tmp, err := os.CreateTemp(dir, "manifest-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// HasSkill returns true if the given skill name exists in the manifest.
func (m *Manifest) HasSkill(name string) bool {
	_, ok := m.Skills[name]
	return ok
}

// UpsertSkill adds or updates a skill entry with the given name and SHA.
// For new entries, InstalledAt and LastSyncedAt are set to now.
// For existing entries, only SHA and LastSyncedAt are updated.
func (m *Manifest) UpsertSkill(name, sha string) {
	now := time.Now()
	if existing, ok := m.Skills[name]; ok {
		existing.SHA = sha
		existing.LastSyncedAt = now
		m.Skills[name] = existing
	} else {
		m.Skills[name] = SkillEntry{
			SHA:          sha,
			InstalledAt:  now,
			LastSyncedAt: now,
		}
	}
}

// RemoveSkill removes a skill from the manifest by name.
func (m *Manifest) RemoveSkill(name string) {
	delete(m.Skills, name)
}

// RebuildFromDisk scans targetDir for directories whose names match repo skills,
// computes their local tree SHA, and returns a Manifest populated from local disk state.
//
// Only directories whose names appear in repoSkills are included in the returned
// manifest. Other local directories are assumed to be non-managed and are ignored.
func RebuildFromDisk(targetDir string, repoSkills []repo.SkillDir) (*Manifest, error) {
	m := &Manifest{
		Version: 1,
		Skills:  make(map[string]SkillEntry),
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("reading target directory: %w", err)
	}

	// Build a set of repo skill names for fast lookup
	repoNames := make(map[string]string, len(repoSkills))
	for _, s := range repoSkills {
		repoNames[s.Name] = s.TreeSHA
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, inRepo := repoNames[name]; !inRepo {
			continue
		}

		fullPath := filepath.Join(targetDir, name)
		sha, err := repo.ComputeLocalTreeSHA(fullPath)
		if err != nil {
			// If we can't compute the SHA (e.g. permission error), skip
			continue
		}

		m.Skills[name] = SkillEntry{
			SHA:          sha,
			InstalledAt:  now,
			LastSyncedAt: now,
		}
	}

	return m, nil
}

// RepairCorrupt moves a corrupt manifest file to a .bak backup.
// If the file does not exist, it is a no-op.
func RepairCorrupt(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	backup := path + ".bak"
	if err := os.WriteFile(backup, data, 0644); err != nil {
		return fmt.Errorf("writing backup: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing corrupt file: %w", err)
	}

	return nil
}

// emptyManifest returns a newly initialised, empty Manifest.
func emptyManifest() *Manifest {
	return &Manifest{
		Version: 1,
		Skills:  make(map[string]SkillEntry),
	}
}
