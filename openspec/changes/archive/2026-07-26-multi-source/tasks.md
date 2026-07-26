# Tasks: Multi-Source Skill Repos

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~900-1100 |
| 800-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Config/manifest → PR 2: `slap source` → PR 3: Source-aware commands |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
800-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Config types, SourceConfig CRUD, Source field on SkillEntry, composite key, migration logic | PR 1 | `go test ./internal/config/... ./internal/manifest/...` | N/A — lib-only, no CLI entrypoint | revert config.go + manifest.go changes |
| 2 | `cmd/source.go` (add/list/remove), migration trigger on first slap source/sync, register source subcommand in root | PR 2 | `go test ./cmd/... -run Source` | N/A — manual trigger, no E2E infra | revert cmd/source.go + root.go |
| 3 | Source-aware sync/install loops, display as `name (alias)` in list/status/remove, `--source` flag | PR 3 | `go test ./...` | N/A — multi-source E2E needs repos | revert all cmd changes except source.go |

## Phase 1: Foundation (Config + Manifest)

- [x] 1.1 Add `SourceConfig` struct, `Sources []string` to `Config` in `internal/config/config.go`
- [x] 1.2 Add `SourcesDir()` helper, source CRUD (Create/Read/Delete source file) in `internal/config/`
- [x] 1.3 Add `MigrateConfig()` — detect old `repo_url`, backup config, write `sources/default.yaml`, rewrite config
- [x] 1.4 Add `Source` field to `SkillEntry` in `internal/manifest/manifest.go`
- [x] 1.5 Add `manifestKey(source, name)` composite-key helper, update `HasSkill`/`RemoveSkill` to accept source
- [x] 1.6 Add source-filtered methods: `SkillsBySource()`, `RemoveBySource()`

## Phase 2: New `slap source` Command

- [x] 2.1 Create `cmd/source.go` with `slap source add` — prompt URL + alias, validate reachable, check uniqueness, write source file
- [x] 2.2 Implement `slap source list` — read sources dir, display alias/URL/branch
- [x] 2.3 Implement `slap source remove <alias>` — prompt uninstall of source's skills, delete source file, remove skills if confirmed
- [x] 2.4 Wire migration trigger: detect old config before first `source` or `sync` command
- [x] 2.5 Register `source` subcommand in `cmd/root.go`

## Phase 3: Source-Aware Sync + Install

- [x] 3.1 Update `internal/sync/sync.go` — accept source param, filter manifest by source in `Plan`
- [x] 3.2 Rewrite `cmd/sync.go` — loop over all sources, per-source clone + reconcile, per-source error isolation
- [x] 3.3 Update `cmd/install.go` — add `--source` flag, install-available-list from ALL sources grouped by alias
- [x] 3.4 Per-source temp clone dirs (isolated per alias)

## Phase 4: Source-Aware Display + Remove

- [x] 4.1 Update `cmd/list.go` — display `name (alias)`, include `source` field in `--json`
- [x] 4.2 Update `cmd/status.go` — source-aware drift reporting
- [x] 4.3 Update `cmd/remove.go` — accept `source:name` syntax, source-aware skill removal

## Phase 5: Testing

- [x] 5.1 Unit tests: manifest keying, `SkillsBySource`, source-filtered `HasSkill`/`RemoveSkill`
- [x] 5.2 Unit tests: `MigrateConfig` with mock old config — verify backup + sources file
- [x] 5.3 Unit tests: source CRUD (Create/Read/Delete)
- [x] 5.4 Unit tests: `Plan` with source param in sync
- [x] 5.5 Integration: source add/list/remove end-to-end with temp dir
- [x] 5.6 Integration: sync across 2 sources with temp repos
- [x] 5.7 Verify full flow: `go build ./...` + `go test -v -count=1 ./...`
