package sync

import (
	"testing"
	"time"

	"github.com/vekzz-dev/skills-manager/internal/manifest"
	"github.com/vekzz-dev/skills-manager/internal/repo"
)

// makeManifest is a helper to create a manifest with the given skills.
func makeManifest(skills map[string]string) *manifest.Manifest {
	m := &manifest.Manifest{
		Version: 1,
		Skills:  make(map[string]manifest.SkillEntry),
	}
	now := time.Now()
	for name, sha := range skills {
		m.Skills[name] = manifest.SkillEntry{
			SHA:          sha,
			InstalledAt:  now,
			LastSyncedAt: now,
		}
	}
	return m
}

// makeRepoSKills converts a map[name]sha to []repo.SkillDir.
func makeRepoSkills(m map[string]string) []repo.SkillDir {
	var skills []repo.SkillDir
	for name, sha := range m {
		skills = append(skills, repo.SkillDir{Name: name, TreeSHA: sha})
	}
	return skills
}

// makeLocalSHAs converts a map[name]sha to local SHAs.
func makeLocalSHAs(m map[string]string) map[string]string {
	return m
}

func actionsToString(actions []Action) []string {
	var s []string
	for _, a := range actions {
		s = append(s, a.Name+":"+string(a.Type))
	}
	return s
}

func containsAction(actions []Action, name string, actionType ActionType) bool {
	for _, a := range actions {
		if a.Name == name && a.Type == actionType {
			return true
		}
	}
	return false
}

func TestPlanAddNewSkill(t *testing.T) {
	m := makeManifest(nil) // empty manifest
	repoSk := makeRepoSkills(map[string]string{"new-skill": "abc123"})
	local := makeLocalSHAs(nil)

	actions := Plan(m, repoSk, local, false)

	if !containsAction(actions, "new-skill", ActionAdd) {
		t.Errorf("expected add action for new-skill, got %v", actionsToString(actions))
	}
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d: %v", len(actions), actionsToString(actions))
	}
}

func TestPlanUpdateChangedSkill(t *testing.T) {
	m := makeManifest(map[string]string{"skill-a": "old-sha"})
	repoSk := makeRepoSkills(map[string]string{"skill-a": "new-sha"})
	local := makeLocalSHAs(map[string]string{"skill-a": "old-sha"})

	actions := Plan(m, repoSk, local, false)

	if !containsAction(actions, "skill-a", ActionUpdate) {
		t.Errorf("expected update action for skill-a, got %v", actionsToString(actions))
	}
}

func TestPlanSkipUnchangedSkill(t *testing.T) {
	m := makeManifest(map[string]string{"skill-a": "same-sha"})
	repoSk := makeRepoSkills(map[string]string{"skill-a": "same-sha"})
	local := makeLocalSHAs(map[string]string{"skill-a": "same-sha"})

	actions := Plan(m, repoSk, local, false)

	if !containsAction(actions, "skill-a", ActionSkip) {
		t.Errorf("expected skip action for skill-a, got %v", actionsToString(actions))
	}
}

func TestPlanRemoveWithPrune(t *testing.T) {
	m := makeManifest(map[string]string{"removed-skill": "abc"})
	repoSk := makeRepoSkills(nil)
	local := makeLocalSHAs(nil)

	actions := Plan(m, repoSk, local, true)

	if !containsAction(actions, "removed-skill", ActionRemove) {
		t.Errorf("expected remove action for removed-skill with --prune, got %v", actionsToString(actions))
	}
}

func TestPlanSkipRemoveWithoutPrune(t *testing.T) {
	m := makeManifest(map[string]string{"removed-skill": "abc"})
	repoSk := makeRepoSkills(nil)
	local := makeLocalSHAs(nil)

	actions := Plan(m, repoSk, local, false)

	if !containsAction(actions, "removed-skill", ActionSkip) {
		t.Errorf("expected skip action for removed-skill without --prune, got %v", actionsToString(actions))
	}
}

func TestPlanLocalModNoRepoChange(t *testing.T) {
	m := makeManifest(map[string]string{"skill-a": "manifest-sha"})
	repoSk := makeRepoSkills(map[string]string{"skill-a": "manifest-sha"})
	local := makeLocalSHAs(map[string]string{"skill-a": "local-modified-sha"})

	actions := Plan(m, repoSk, local, false)

	if !containsAction(actions, "skill-a", ActionLocalModNoRepoChange) {
		t.Errorf("expected local-mod-no-repo-change for skill-a, got %v", actionsToString(actions))
	}
}

func TestPlanLocalModWithRepoUpdate(t *testing.T) {
	m := makeManifest(map[string]string{"skill-a": "manifest-sha"})
	repoSk := makeRepoSkills(map[string]string{"skill-a": "new-repo-sha"})
	local := makeLocalSHAs(map[string]string{"skill-a": "local-modified-sha"})

	actions := Plan(m, repoSk, local, false)

	if !containsAction(actions, "skill-a", ActionLocalModWithRepoUpdate) {
		t.Errorf("expected local-mod-with-repo-update for skill-a, got %v", actionsToString(actions))
	}
}

func TestPlanEmptyManifest(t *testing.T) {
	m := makeManifest(nil)
	repoSk := makeRepoSkills(map[string]string{"skill-a": "abc", "skill-b": "def"})
	local := makeLocalSHAs(nil)

	actions := Plan(m, repoSk, local, false)

	if len(actions) != 2 {
		t.Errorf("expected 2 actions (add for each repo skill), got %d: %v", len(actions), actionsToString(actions))
	}

	if !containsAction(actions, "skill-a", ActionAdd) {
		t.Error("missing add for skill-a")
	}
	if !containsAction(actions, "skill-b", ActionAdd) {
		t.Error("missing add for skill-b")
	}
}

func TestPlanMixedState(t *testing.T) {
	m := makeManifest(map[string]string{
		"add-me":       "does-not-exist-in-repo", // repo doesn't have it in this test — will be skip/remove
		"update-me":    "old-sha",
		"skip-me":      "same-sha",
		"remove-me":    "any-sha",
	})
	repoSk := makeRepoSkills(map[string]string{
		"update-me":  "new-sha",
		"skip-me":    "same-sha",
		"add-me":     "repo-sha", // in repo but not in manifest? No, it IS in manifest, but with a wrong hash
		"brand-new":  "fresh-sha",
	})
	local := makeLocalSHAs(map[string]string{
		"update-me": "old-sha",
		"skip-me":   "same-sha",
		"add-me":    "does-not-exist-in-repo",
	})

	actions := Plan(m, repoSk, local, false)

	checks := []struct {
		name string
		typ  ActionType
	}{
		// "add-me": in manifest with sha "does-not-exist-in-repo", but repo has it as "repo-sha"
		// localSHA "does-not-exist-in-repo" != manifestSHA "does-not-exist-in-repo"??? wait they're the same
		// Actually manifest says sha "does-not-exist-in-repo", local says same, repo says "repo-sha"
		// So localSHA == manifestSHA, repo != manifest → update
		{"add-me", ActionUpdate},
		{"update-me", ActionUpdate},
		{"skip-me", ActionSkip},
		{"brand-new", ActionAdd},
	}

	for _, c := range checks {
		if !containsAction(actions, c.name, c.typ) {
			t.Errorf("expected action %s for %s in mixed state, got %v",
				c.typ, c.name, actionsToString(actions))
		}
	}

	// "remove-me" is in manifest but not in repo → skip (no prune)
	if !containsAction(actions, "remove-me", ActionSkip) {
		t.Errorf("expected skip for remove-me (no prune), got %v", actionsToString(actions))
	}
}

func TestPlanMixedStateWithPrune(t *testing.T) {
	m := makeManifest(map[string]string{
		"keep":      "same-sha",
		"delete-me": "abc",
	})
	repoSk := makeRepoSkills(map[string]string{
		"keep": "same-sha",
	})
	local := makeLocalSHAs(map[string]string{
		"keep": "same-sha",
	})

	actions := Plan(m, repoSk, local, true)

	if !containsAction(actions, "keep", ActionSkip) {
		t.Errorf("expected skip for keep, got %v", actionsToString(actions))
	}
	if !containsAction(actions, "delete-me", ActionRemove) {
		t.Errorf("expected remove for delete-me with --prune, got %v", actionsToString(actions))
	}
}

func TestPlanLocalSHAEmptyMeansMissingFolder(t *testing.T) {
	// When localSHA is not in the map (folder missing), treat as no local modification
	m := makeManifest(map[string]string{"skill-a": "manifest-sha"})
	repoSk := makeRepoSkills(map[string]string{"skill-a": "new-repo-sha"})
	local := makeLocalSHAs(map[string]string{}) // skill-a not present locally

	actions := Plan(m, repoSk, local, false)

	// Since local is missing entirely, we fall to: repo != manifest → update
	if !containsAction(actions, "skill-a", ActionUpdate) {
		t.Errorf("expected update for skill-a when local folder missing, got %v", actionsToString(actions))
	}
}

func TestPlanNilManifest(t *testing.T) {
	actions := Plan(nil, nil, nil, false)
	if len(actions) != 0 {
		t.Errorf("Plan(nil) should return 0 actions, got %d", len(actions))
	}
}

func TestActionConstants(t *testing.T) {
	if ActionAdd != "add" {
		t.Errorf("ActionAdd = %q, want 'add'", ActionAdd)
	}
	if ActionUpdate != "update" {
		t.Errorf("ActionUpdate = %q, want 'update'", ActionUpdate)
	}
	if ActionRemove != "remove" {
		t.Errorf("ActionRemove = %q, want 'remove'", ActionRemove)
	}
	if ActionSkip != "skip" {
		t.Errorf("ActionSkip = %q, want 'skip'", ActionSkip)
	}
	if ActionLocalModNoRepoChange != "local-mod-no-repo-change" {
		t.Errorf("ActionLocalModNoRepoChange = %q, want 'local-mod-no-repo-change'", ActionLocalModNoRepoChange)
	}
	if ActionLocalModWithRepoUpdate != "local-mod-with-repo-update" {
		t.Errorf("ActionLocalModWithRepoUpdate = %q, want 'local-mod-with-repo-update'", ActionLocalModWithRepoUpdate)
	}
}
