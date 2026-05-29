// Package sync computes the delta between manifest, local disk, and repo states.
package sync

import (
	"github.com/vekzz-dev/slap-skills/internal/manifest"
	"github.com/vekzz-dev/slap-skills/internal/repo"
)

// ActionType classifies a sync action.
type ActionType string

const (
	ActionAdd                    ActionType = "add"
	ActionUpdate                 ActionType = "update"
	ActionRemove                 ActionType = "remove"
	ActionSkip                   ActionType = "skip"
	ActionLocalModNoRepoChange   ActionType = "local-mod-no-repo-change"
	ActionLocalModWithRepoUpdate ActionType = "local-mod-with-repo-update"
)

// Action represents a single reconciliation action for one skill.
type Action struct {
	Name     string
	Type     ActionType
	FromSHA  string
	ToSHA    string
	LocalSHA string
}

// Plan computes the delta between manifest state, local disk state, and repo
// state. It is a pure function — no side effects — and is fully testable.
//
// Comparison matrix:
//
//	in manifest? | in repo? | localSHA == manifestSHA? | result
//	no           | yes      | —                        | add
//	yes          | yes      | yes, repo==manifest      | skip
//	yes          | yes      | yes, repo!=manifest      | update
//	yes          | yes      | no,  repo==manifest      | localModNoRepoChange (warn)
//	yes          | yes      | no,  repo!=manifest      | localModWithRepoUpdate (warn+overwrite)
//	yes          | no       | —                        | remove (if prune), else skip
//	no           | no       | —                        | ignore
func Plan(m *manifest.Manifest, repoSkills []repo.SkillDir, localSHAs map[string]string, prune bool) []Action {
	var actions []Action

	if m == nil {
		return actions
	}

	// Build a repo skill map for O(1) lookup
	repoMap := make(map[string]repo.SkillDir, len(repoSkills))
	for _, s := range repoSkills {
		repoMap[s.Name] = s
	}

	// Evaluate each skill in the manifest
	for name, entry := range m.Skills {
		repoSkill, inRepo := repoMap[name]
		if !inRepo {
			// In manifest but absent from repo
			if prune {
				actions = append(actions, Action{
					Name:    name,
					Type:    ActionRemove,
					FromSHA: entry.SHA,
				})
			} else {
				actions = append(actions, Action{
					Name:    name,
					Type:    ActionSkip,
					FromSHA: entry.SHA,
				})
			}
			continue
		}

		// In both manifest and repo — compare SHAs
		repoSHA := repoSkill.TreeSHA
		localSHA, localExists := localSHAs[name]
		manifestSHA := entry.SHA

		if localExists && localSHA != manifestSHA {
			// Local modification detected
			if repoSHA != manifestSHA {
				// Both repo and local differ from manifest
				actions = append(actions, Action{
					Name:     name,
					Type:     ActionLocalModWithRepoUpdate,
					FromSHA:  manifestSHA,
					ToSHA:    repoSHA,
					LocalSHA: localSHA,
				})
			} else {
				// Repo hasn't changed, only local differs
				actions = append(actions, Action{
					Name:     name,
					Type:     ActionLocalModNoRepoChange,
					FromSHA:  manifestSHA,
					ToSHA:    repoSHA,
					LocalSHA: localSHA,
				})
			}
		} else if repoSHA != manifestSHA {
			// Repo has a new version — normal update
			actions = append(actions, Action{
				Name:    name,
				Type:    ActionUpdate,
				FromSHA: manifestSHA,
				ToSHA:   repoSHA,
			})
		} else {
			// Everything in sync
			actions = append(actions, Action{
				Name:    name,
				Type:    ActionSkip,
				FromSHA: manifestSHA,
				ToSHA:   repoSHA,
			})
		}
	}

	// Detect new skills in repo that are not in manifest
	for _, s := range repoSkills {
		if _, inManifest := m.Skills[s.Name]; !inManifest {
			actions = append(actions, Action{
				Name:  s.Name,
				Type:  ActionAdd,
				ToSHA: s.TreeSHA,
			})
		}
	}

	return actions
}
