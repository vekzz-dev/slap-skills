# Tasks: Skills CLI (Slap Skills)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1700 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Foundation + Sync → PR 2: CLI Commands → PR 3: Release |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | config, manifest, repo, sync + tests | PR 1 | Core engine, fully testable without CLI |
| 2 | cmd/*, main.go — CLI commands | PR 2 | Wires commands to engine |
| 3 | .goreleaser.yaml, CI workflow | PR 3 | CI/CD only, no runtime changes |

## Phase 1: Foundation

- [x] 1.1 `go.mod` — init module with cobra, go-git/v5, gopkg.in/yaml.v3
- [x] 1.2 `internal/config/config.go` — Config struct, Load/Save, ApplyFlagOverrides, ValidateRepoAccess
- [x] 1.3 `internal/config/config_test.go` — test load, save, flag overrides, missing file
- [x] 1.4 `internal/manifest/manifest.go` — Manifest, SkillEntry, Load (corrupt→backup), Save (atomic), Upsert/Remove
- [x] 1.5 `internal/manifest/manifest_test.go` — marshal, atomic save, corrupt JSON, missing file
- [x] 1.6 `internal/repo/repo.go` — Client, CloneShallow, ListSkillDirs, ComputeLocalTreeSHA
- [x] 1.7 `internal/repo/repo_test.go` — integration tests with in-memory go-git repos

## Phase 2: Sync Engine

- [x] 2.1 `internal/sync/sync.go` — Plan function (pure, 3-input delta), Action types, all comparison combos
- [x] 2.2 `internal/sync/sync_test.go` — table-driven tests: add/update/remove/skip/local-mod combos

## Phase 3: CLI Commands

- [ ] 3.1 `cmd/root.go` — Cobra root command, binary name `slap`
- [ ] 3.2 `cmd/init.go` — init: validate repo, write config, output guidance
- [ ] 3.3 `cmd/sync.go` — sync: config→clone→plan→execute→manifest write
- [ ] 3.4 `cmd/list.go` — list: table or --json output from manifest
- [ ] 3.5 `cmd/status.go` — status: clone, compare, classify drift per skill
- [ ] 3.6 `main.go` — entry point, calls cmd.Execute()

## Phase 4: Release Pipeline

- [ ] 4.1 `.goreleaser.yaml` — 4 targets (linux amd64/arm64, darwin amd64/arm64), brews stanza
- [ ] 4.2 `.github/workflows/release.yml` — trigger on v* tags, run goreleaser
