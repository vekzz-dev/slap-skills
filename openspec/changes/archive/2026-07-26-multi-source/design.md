# Design: Multi-Source Skill Repos

## Technical Approach

Replace single-repo config with a multi-source system. Each source gets its own YAML file under `~/.config/slap/sources/<alias>.yaml`. Manifest gains a `Source` field on `SkillEntry`, keyed internally by `source:name` composite. Migration auto-converts old configs. Sync/install loop over all sources independently with per-source error isolation.

## Architecture Decisions

### Decision: Composite Manifest Key

**Choice**: Internal key as `source:name` (e.g. `work:foo`), display as `foo (work)`
**Alternatives**: Nested map `map[string]map[string]SkillEntry` — cleaner but breaks manifest JSON shape and all callers.
**Rationale**: Flat `Skills` map with `source:name` key keeps backward-compatible JSON structure. `HasSkill`, `RemoveSkill` gain a `source` param. Existing callers that don't care about source (remove-by-name) need migration, but it's one param per function — acceptable cost.

### Decision: Source Config as Separate Files

**Choice**: `~/.config/slap/sources/<alias>.yaml` — one file per source
**Alternatives**: Single `config.yaml` with a list of sources
**Rationale**: File-per-source makes add/remove atomic (create/delete a file). No need to parse/rewrite a list in `config.yaml`. The list-of-active-sources stays in `config.yaml` as a `Sources` field.

### Decision: Per-Source Clone Dirs

**Choice**: Clone each source to its own temp dir (`slap-sync-<alias>-*`)
**Alternatives**: Clone all to shared temp dir with subdirs, or reuse cached clones
**Rationale**: Each clone is independent and disposable. Per-source temp dirs prevent cross-contamination. A cached clone optimization is deferred until needed.

### Decision: Migration as First-Run Trigger

**Choice**: Migration runs on first `slap source` or `slap sync` when `repo_url` exists in config
**Alternatives**: Require explicit `slap migrate` command, or migrate on `slap init`
**Rationale**: Zero-touch upgrade. Old config is detected, backed up, and migrated automatically. The `sources/default.yaml` preserves the original URL so the user sees no behavioral change.

## Data Flow

```
slap sync / slap install
    │
    ├─ Load config.yaml → get Sources list
    ├─ For each source alias:
    │   ├─ Read sources/<alias>.yaml (url, branch)
    │   ├─ Clone to temp dir (isolated)
    │   ├─ List skill dirs
    │   ├─ Plan delta against manifest (filtered by source)
    │   └─ Execute plan (copy, update manifest)
    │
    ├─ Merge all per-source plans
    └─ Save manifest

slap source add <url> [--alias]
    │
    ├─ Validate URL reachable
    ├─ Check alias uniqueness
    └─ Write sources/<alias>.yaml

slap source remove <alias>
    │
    ├─ Confirm uninstall of alias's skills (optional)
    ├─ Remove skills if confirmed
    └─ Delete sources/<alias>.yaml
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config.go` | Modify | Add `Sources []string`, add `SourceConfig` struct, add sources CRUD, migrate from old format |
| `internal/manifest/manifest.go` | Modify | Add `Source` field to `SkillEntry`, key by `source:name`, add source-filtered methods |
| `internal/sync/sync.go` | Modify | Accept source param, filter manifest by source in Plan |
| `cmd/source.go` | Create | New `slap source` command with add/list/remove subcommands |
| `cmd/sync.go` | Modify | Loop over all sources, per-source clone + reconcile, error isolation |
| `cmd/install.go` | Modify | Add `--source` flag, source-aware listing and install |
| `cmd/list.go` | Modify | Display `name (alias)` format, add `source` field in JSON |
| `cmd/status.go` | Modify | Source-aware drift reporting |
| `cmd/remove.go` | Modify | Source-aware remove, support `source:name` syntax |
| `cmd/root.go` | Modify | Register `source` subcommand |

## Interfaces / Contracts

```go
// SourceConfig per source file (~/.config/slap/sources/<alias>.yaml)
type SourceConfig struct {
    Alias  string `yaml:"alias"`
    URL    string `yaml:"url"`
    Branch string `yaml:"branch"`
}

// Updated Config
type Config struct {
    Sources    []string `yaml:"sources"`     // active source aliases
    TargetDir  string   `yaml:"target_dir"`
}

// Updated SkillEntry
type SkillEntry struct {
    Source       string    `json:"source"`
    SHA          string    `json:"sha"`
    InstalledAt  time.Time `json:"installed_at"`
    LastSyncedAt time.Time `json:"last_synced_at"`
}

// Internal key format: source:name
func manifestKey(source, name string) string {
    return source + ":" + name
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Manifest keying, source filtering, Plan with source param | Table-driven tests for `Plan`, `HasSkill`, `RemoveSkill` |
| Unit | Config migration (old→new format) | Test `MigrateConfig` with mock old config, verify backup + sources file |
| Integration | Source add/list/remove end-to-end | Temp dir, create sources dir, verify file CRUD |
| Integration | Sync across 2 sources | Clone two test repos, verify per-source manifest entries |
| E2E | Full flow: init→migrate→add source→sync→list | Requires two repos, temp home dir |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

1. Detect old config (`repo_url` exists, `sources/` dir missing)
2. Backup `config.yaml` → `config.yaml.bak`
3. Create `sources/default.yaml` with URL, branch from old config
4. Rewrite `config.yaml` — remove `repo_url`/`branch`, add `sources: ["default"]`
5. Update manifest: all entries get `Source: "default"`
6. Rollback: restore `config.yaml.bak`, delete `sources/`, revert manifest

## Open Questions

- [x] `slap install` without `--source` — list available skills from ALL sources, grouped by source
