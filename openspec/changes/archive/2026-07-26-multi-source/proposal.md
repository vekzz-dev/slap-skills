# Proposal: Multi-Source Skill Repos

## Intent

Single-repo model limits users who want skills from multiple authors/orgs. Each team or community maintains its own skill repo; users should mix and match without losing origin tracking or conflict resolution.

## Scope

### In Scope
- `slap source` command: interactive add/list/remove sources with named aliases
- Per-source config: each source has alias, URL, branch (stored under `~/.config/slap/sources/`)
- Auto-migration: old `config.yaml` with single `repo_url` becomes source `default`
- Skill display: always shown as `skill-name (source-alias)` everywhere (list, install, sync, status, remove)
- Name conflict handling: same skill from different sources coexists via alias disambiguation
- Source removal: user prompted whether to uninstall that source's skills
- Install, sync, list, status, remove — all source-aware

### Out of Scope
- Source-to-source priority/override rules (deferred)
- Source renaming (deferred — remove + re-add)
- Remote source discovery or registry
- Per-source filter/inclusion patterns

## Capabilities

### New Capabilities
- `source-management`: Add, list, remove skill sources with named aliases via `slap source`
- `multi-source-install`: Install skills from multiple sources with per-skill origin tracking
- `multi-source-sync`: Sync skills across all configured sources, detecting per-source state

### Modified Capabilities
- None — first real base specs for this project (prior specs were child specs of `skills-cli` change)

## Approach

1. **Config**: Single `config.yaml` → `sources/<alias>.yaml` per source (alias, URL, branch). `config.yaml` keeps metadata (target dir, sources list).
2. **Manifest**: Add `Source` field to `SkillEntry`. Skills keyed by `name` only — alias is display-only. Same name + different source = separate entries via manifest-internal dedup key `source:name`.
3. **Migration**: On first `slap source` or `slap sync`, detect old single-repo config. If found, create `sources/default.yaml`, re-write config, and update manifest with `Source: "default"` on all entries.
4. **`slap source`**: `survey.MultiSelect` for operations. Add asks URL + alias, validates access, writes source file. List reads sources dir. Remove asks user whether to uninstall that source's skills.
5. **Display**: All command output renders skills as `name (alias)`. `list --json` includes `source` field.
6. **Sync/Install**: Loop over all configured sources, clone each, reconcile per-source entries. Manifest tracks per-skill source.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/source.go` | New | `slap source` command with add/list/remove |
| `cmd/install.go` | Modified | Source-aware skill listing + install |
| `cmd/sync.go` | Modified | Loop over all sources, per-source reconciliation |
| `cmd/list.go` | Modified | Render `name (alias)` format |
| `cmd/remove.go` | Modified | Source-aware remove |
| `internal/config/` | Modified | Sources system (multi-file), migration logic |
| `internal/manifest/` | Modified | `Source` field on `SkillEntry`, key by `source:name` |
| `internal/sync/` | Modified | Per-source plan + execute |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Migration breaks existing users | Low | Validate old config before migrating; backup original `config.yaml` |
| Same skill name from 2 sources confuses display | Low | Always disambiguate via `name (alias)` — no naked names |
| Partial source clone fails; others succeed | Med | Per-source independent errors; report per-source summary |

## Rollback Plan

1. Restore backed-up `config.yaml` from migration
2. Remove `sources/` dir
3. Re-run `slap sync` to return to single-source mode
4. Script: `slap-multi-source-rollback.sh` in repo docs

## Dependencies

- Existing `go-git`, `cobra`, `survey`, `yaml.v3` — no new deps

## Success Criteria

- [ ] `slap source add --alias work https://github.com/org/skills` adds source and validates access
- [ ] `slap source list` shows all configured sources with aliases
- [ ] `slap source remove work` prompts to uninstall work's skills, then deletes source
- [ ] Old `config.yaml` auto-migrates to `default` source on first command
- [ ] `slap list` shows `foo (default)`, `foo (work)` for same-named skills
- [ ] Re-running sync across all sources is a no-op when nothing changed
