# Proposal: slap-skills

## Intent

A generic CLI tool ("Slap Skills") that lets any user manage their own opencode skills from a git repo (public or private). Instead of manually copying folders to `~/.config/opencode/skills/`, the tool syncs skills from the configured repo — adding new ones, updating changed ones, and optionally removing deleted ones — without touching skills from other sources (SDD, branch-pr, etc.). The repo URL is user-configurable, making it usable by anyone, not tied to a specific account.

## Scope

### In Scope
- `init <repo-url>` command: configure the tool with a git repo URL (stored in `~/.config/slap/config.yaml`)
- `sync` command: shallow-clone repo, reconcile local skills against manifest, add/update/remove skill folders
- `list` command: show installed skills from the manifest with origin info
- `status` command: compare local skills vs repo HEAD, show drift (newer/changed/missing)
- Manifest file (`~/.config/slap/manifest.json`) to track installed skills per origin repo
- Origin isolation: only skills from the configured repo are managed; all other skill folders are untouched
- Config file (`~/.config/slap/config.yaml`) to persist repo URL, branch, and target directory
- Support for public AND private repos (uses git credentials from the system)
- Release pipeline: GoReleaser config for cross-platform builds + GitHub Releases
- Homebrew formula: installable via `brew install slap-skills` (or custom tap)

### Out of Scope
- Pushing or contributing changes back to the repo
- Supporting multiple repos simultaneously (deferred)
- Daemon/watch mode (deferred)
- Installing individual skills by name without a full sync
- Validating or running skill content
- Publishing to non-Homebrew package managers (apt, Scoop, etc.)
- Web UI or GUI

## Capabilities

### New Capabilities
- `skills-init`: Configure the tool with a git repo URL — stores to `~/.config/slap/config.yaml`, validates repo is accessible
- `skills-sync`: Pull skills from the configured GitHub repo and reconcile local installation (add/update/remove) based on manifest state
- `skills-list`: Enumerate installed skills from the manifest with metadata (name, version/sha, install date)
- `skills-status`: Compare local skill version against repo HEAD and report drift (ahead, behind, new, missing)
- `release-pipeline`: GoReleaser config + GitHub Actions to build and publish cross-platform binaries
- `homebrew-formula`: Homebrew formula for easy installation

### Modified Capabilities
- None — this is a greenfield change with no existing specs

## Approach

CLI binary (`slap`) using cobra for command routing.

Config flow:
1. `slap init <repo-url>` validates the repo is accessible (git ls-remote), stores repo URL + branch in `~/.config/slap/config.yaml`

Sync flow:
1. Read config from `~/.config/slap/config.yaml` (repo URL, branch)
2. Shallow-clone repo via go-git into a temp dir
3. Read manifest from `~/.config/slap/manifest.json`
4. For each skill folder in repo: compare tree SHA vs manifest → add new or update changed
5. If `--prune` flag: remove local skills in manifest but absent from repo
6. Write updated manifest (atomic temp file + rename)

Package layout: `cmd/` for cobra entrypoints, `internal/sync/` for sync logic, `internal/manifest/` for manifest read/write, `internal/config/` for config load/save.

Release: GoReleaser builds linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 binaries. Each release creates a GitHub Release with artifacts. A Homebrew formula references the latest release archive and SHA.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `main.go` | New | Cobra root command + subcommands |
| `cmd/init.go` | New | Init command — validates repo, writes config |
| `cmd/sync.go` | New | Sync command |
| `cmd/list.go` | New | List command |
| `cmd/status.go` | New | Status command |
| `internal/sync/` | New | Sync reconciliation logic |
| `internal/manifest/` | New | Manifest file read/write/merge |
| `internal/config/` | New | Config load/save (YAML) |
| `~/.config/slap/config.yaml` | New (runtime) | User config file with repo URL, branch, target dir |
| `.goreleaser.yaml` | New | Release build config |
| `go.mod` | Modified | Add cobra, go-git, gopkg.in/yaml.v3 dependencies |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Network failure during sync | Medium | Retry with backoff; report partial state; never leave target half-updated |
| Git auth failure (private repos) | Low | Document SSH/HTTPS auth setup; `git clone` uses existing git credentials |
| Partial sync interrupted | Low | Manifest update is atomic (write temp file + rename); next sync resumes from manifest state |
| Manifest schema drift | Low | Version the manifest schema; on mismatch, warn and rebuild from disk state |

## Rollback Plan

1. Manifest stores `previous_state` backup on each sync
2. If a sync fails mid-operation: re-run `sync` — it resumes from manifest state (unchanged skills untouched, partially updated ones get re-synced)
3. To full revert: `rm -rf ~/.config/opencode/skills/<skills-from-repo> && rm ~/.config/slap/manifest.json` — non-managed skills are unaffected by either operation

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- Go standard library (`os/exec` for git) or `github.com/go-git/go-git/v5` — git operations
- GitHub repo `https://github.com/vekzz-dev/opencode-skills` must exist and be accessible
- `github.com/goreleaser/goreleaser` — release pipeline (CI only, not a runtime dep)
- Homebrew tap repo (e.g. `github.com/vekzz-dev/homebrew-tap`) for the formula

## Success Criteria

- [ ] `slap init https://github.com/user/skills` creates `~/.config/slap/config.yaml` and validates the repo
- [ ] `slap sync` installs all skills from the configured repo to `~/.config/opencode/skills/`
- [ ] `slap list` shows installed skills with origin metadata
- [ ] `slap status` reports diff between local and repo HEAD
- [ ] Skills from non-managed sources (SDD, etc.) are never modified or deleted
- [ ] Re-running `sync` on unchanged repo is a no-op (manifest matches)
- [ ] Re-running `sync` after adding/removing a skill folder in the repo correctly updates local state
- [ ] `goreleaser --snapshot` produces correct binaries for all target platforms
- [ ] `brew install slap-skills` installs and runs the tool successfully
