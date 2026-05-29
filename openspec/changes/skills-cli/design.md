# Design: Slap Skills

## Technical Approach

A Cobra-based Go CLI (`slap`) that syncs opencode skills from any user-configured git repo (public or private) to `~/.config/opencode/skills/`. The tool has a config file at `~/.config/slap/config.yaml` to persist the repo URL, and a manifest at `~/.config/slap/manifest.json` to track installed skills by folder tree SHA. `init` configures the repo; `sync` shallow-clones the repo and reconciles local state; `list` reads the manifest; `status` shallow-clones and reports per-skill drift.

Specs: `skills-init`, `skills-sync`, `skills-list`, `skills-status`, `release-pipeline`, `homebrew-formula`.

## Architecture Decisions

### Decision: go-git/v5 for git operations

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `os/exec` git | Simple shell outs; relies on system git; hard to test without real repos | ❌ Rejected |
| **go-git/v5** | Heavier dep (~8MB) but pure Go, no git prerequisite, in-memory repos for tests | ✅ Chosen |

**Rationale**: Portability is the CLI's selling point — requiring git defeats it. go-git's in-memory repository support enables fast, hermetic integration tests for clone, tree listing, and SHA comparison without network or filesystem setup. GitHub compatibility with go-git is well-established.

### Decision: Per-skill folder tree SHA in manifest

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Repo-level commit SHA | Single SHA for all skills; `status` can't report per-skill drift (all "behind" or "current") | ❌ Rejected |
| **Per-skill tree SHA** | Granular per-folder detection; `status` reports exactly which skills changed; also enables local modification detection | ✅ Chosen |

**Rationale**: The `status` spec requires per-skill drift classification (up-to-date / behind / new / missing). A repo-level SHA cannot distinguish which skill folder changed in a new commit. Tree SHAs from `git ls-tree HEAD` (or go-git's `commit.Tree().Entries`) give us stable, deterministic fingerprints per directory. Additionally, comparing the local folder's current tree SHA against the manifest entry lets us detect skills the user modified directly in the skills directory.

### Decision: YAML config file at ~/.config/slap/config.yaml

| Option | Tradeoff | Decision |
|--------|----------|----------|
| **Config file (YAML)** | ~/.config/slap/config.yaml persists repo URL; init command writes it; sync/status read it | ✅ Chosen |
| Flags only | Requires --repo on every call; no persistence; no `init` flow | ❌ Rejected |

**Rationale**: Since the tool is generic (any user, any repo), the repo URL must persist between invocations. A config file at `~/.config/slap/config.yaml` is the standard XDG location. `init <repo-url>` writes it; all other commands read it. Flags (`--repo`, `--branch`) can override for one-off use.

### Decision: Three-phase sync with recovery and validation

The sync flow has three phases:
1. **Pre-flight**: Load config, load/repair manifest, validate consistency (folders exist, parseable JSON)
2. **Plan**: Compute delta between manifest state, local disk state, and repo state — detects local modifications, missing folders, and new/updated/removed skills
3. **Execute**: Apply the plan atomically — copy, update, remove, then write manifest

This three-phase approach keeps the `Plan` function pure (no side effects, fully testable) and isolates recovery logic from execution. The pre-flight phase handles manifest corruption (backup + rebuild) and consistency repair transparently.

### Decision: go-git shallow clone to temp dir for sync + status

Both `sync` and `status` work by cloning the repo (depth=1) to `os.TempDir()`. For `status`, the clone is discarded after listing the tree. For `sync`, skill files are copied from the clone to the target directory. Temp dirs are cleaned up on completion (defer `os.RemoveAll`). This avoids mixing internal git metadata with user skill files.

## Data Flow

### init flow
```
slap init <repo-url>
  → repo.ValidateAccess(url, branch)  (go-git ls-remote or clone --dry-run)
  → config.Save(~/.config/slap/config.yaml)
  → Print: "Slap configured! Run 'slap sync' to install skills."
```

### sync flow (three-phase)
```
slap sync [--repo url] [--branch name] [--prune]
                  │
                  ▼
            ┌── PRE-FLIGHT ──────────────────────────────────┐
            │ config.Load(~/.config/slap/config.yaml)        │
            │                                                │
            │ manifest.Load(~/.config/slap/manifest.json)    │
            │   ├── os.ErrNotExist  → empty manifest         │
            │   ├── json.SyntaxError → backup + warn + empty │
            │   └── ok → validate entries:                   │
            │       for each entry:                          │
            │         if folder missing → remove from        │
            │           manifest (will be re-added from repo)│
            └────────────────────────────────────────────────┘
                  │
                  ▼
            repo.CloneShallow(tempDir, url, branch)
                  │
                  ▼
            repo.ListSkillDirs(tempDir)  ──→ []SkillDir{name, treeSHA}
                  │
                  ▼
            ┌── PLAN ────────────────────────────────────────┐
            │ sync.Plan(manifest, repoSkills, localSHAs,     │
            │           prune)                               │
            │                                                │
            │ Computes per-skill:                            │
            │   in manifest? in repo? localSHA vs manifest?  │
            │                                                │
            │ Result: []Action{                              │
            │   add / update / remove /                      │
            │   skip / localModNoRepoChange /                 │
            │   localModWithRepoUpdate                       │
            │ }                                              │
            └────────────────────────────────────────────────┘
                  │
                  ▼
            ┌── EXECUTE ─────────────────────────────────────┐
            │ for each add:                                  │
            │   os.CopyFS(tempDir/skill → targetDir/skill)   │
            │ for each update + localModWithRepoUpdate:      │
            │   os.RemoveAll + os.CopyFS (with warning)      │
            │ for each remove:                               │
            │   os.RemoveAll (if --prune)                    │
            │ for each localModNoRepoChange:                 │
            │   print warning, do NOT touch files            │
            │ for each skip:                                 │
            │   nothing                                      │
            │                                                │
            │ manifest.Save(tempFile + rename)               │
            └────────────────────────────────────────────────┘
```

### list flow
```
slap list [--json]
  → manifest.Load(slapDir) → render table or JSON
```

### status flow
```
slap status
  → config.Load → manifest.Load(slapDir) → map[skill]entry{sha}
  → repo.CloneShallow(tempDir, url, branch) → entries{name, treeSHA}
  → Compare → classify per skill (up-to-date | behind | new | missing)
  → Render table to stdout
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `main.go` | Create | Package main entry, calls `cmd.Execute()` |
| `cmd/root.go` | Create | Cobra root command; binary name `slap` |
| `cmd/init.go` | Create | `init` subcommand — validates repo URL, writes `~/.config/slap/config.yaml` |
| `cmd/sync.go` | Create | `sync` subcommand, orchestrates config → clone → plan → copy → manifest write |
| `cmd/list.go` | Create | `list` subcommand with `--json` flag, reads manifest, renders table or JSON |
| `cmd/status.go` | Create | `status` subcommand, shallow clone, compares tree SHAs, renders drift table |
| `internal/config/config.go` | Create | `Config` struct, `Load` from YAML, `Save` to YAML, flag overrides |
| `internal/config/config_test.go` | Create | Tests for load, save, flag overrides |
| `internal/manifest/manifest.go` | Create | `Manifest` type with `Load`, `Save` (atomic write), and `SkillEntry` |
| `internal/manifest/manifest_test.go` | Create | Table-driven tests for marshal/unmarshal, atomic save, missing file |
| `internal/repo/repo.go` | Create | `CloneShallow`, `ListSkillDirs`, `SkillDir` type wrapping go-git |
| `internal/repo/repo_test.go` | Create | Integration tests using go-git in-memory repos |
| `internal/sync/sync.go` | Create | `Plan` function that computes add/update/remove/skip actions |
| `internal/sync/sync_test.go` | Create | Table-driven tests for `Plan` with various manifest/repo states |
| `.goreleaser.yaml` | Create | GoReleaser config: 4 targets, brews stanza for Homebrew tap |
| `.github/workflows/release.yml` | Create | GitHub Actions: triggers on `v*` tags, runs goreleaser |
| `go.mod` | Create | Go module: `github.com/vekzz-dev/slap-skills` |

## Interfaces / Contracts

### Manifest (internal/manifest/manifest.go)

```go
type SkillEntry struct {
    SHA          string    `json:"sha"`           // git tree SHA of skill directory
    InstalledAt  time.Time `json:"installed_at"`
    LastSyncedAt time.Time `json:"last_synced_at"`
}

type Manifest struct {
    Version      int                   `json:"version"`       // schema version, currently 1
    SourceRepo   string                `json:"source_repo"`
    SourceBranch string                `json:"source_branch"`
    LastSync     time.Time             `json:"last_sync"`
    Skills       map[string]SkillEntry `json:"skills"`        // key: skill folder name
}

func Load(path string) (*Manifest, error)            // os.ReadFile + json.Unmarshal; os.ErrNotExist → empty manifest
func (m *Manifest) Save(path string) error            // temp file + os.Rename (atomic)
func (m *Manifest) HasSkill(name string) bool
func (m *Manifest) UpsertSkill(name string, sha string)
func (m *Manifest) RemoveSkill(name string)
```

### Repo (internal/repo/repo.go)

```go
type SkillDir struct {
    Name    string // folder name (e.g. "changelog-maintenance")
    TreeSHA string // git tree object SHA (hex)
}

type Client struct {
    URL    string
    Branch string
}

func (c *Client) CloneShallow(ctx context.Context, dest string) error     // go-git PlainClone with Depth:1
func (c *Client) ListSkillDirs(ctx context.Context, clonePath string) ([]SkillDir, error)
// Reads root tree entries via go-git, filters for type=tree, returns Name + Hash

// ComputeLocalTreeSHA computes a deterministic SHA for a local folder.
// Uses the same algorithm as git: sort entries, hash blobs + subtrees recursively.
// Used for detecting local modifications vs manifest state.
func ComputeLocalTreeSHA(root string) (string, error)
```

### Sync (internal/sync/sync.go)

```go
type ActionType string
const (
    ActionAdd                    ActionType = "add"
    ActionUpdate                 ActionType = "update"
    ActionRemove                 ActionType = "remove"
    ActionSkip                   ActionType = "skip"
    ActionLocalModNoRepoChange   ActionType = "local-mod-no-repo-change"
    ActionLocalModWithRepoUpdate ActionType = "local-mod-with-repo-update"
)

type Action struct {
    Name     string
    Type     ActionType
    FromSHA  string // repo tree SHA (for add/update)
    ToSHA    string // manifest tree SHA (for remove/compare)
    LocalSHA string // current local tree SHA (for local modification detection)
}

// Plan computes the delta between manifest state, local disk state, and repo state.
// Pure computation — no side effects, pure function, fully testable.
//
// Comparison matrix:
//   in manifest? | in repo? | localSHA==manifestSHA? | result
//   no           | yes      | —                      | add
//   yes          | yes      | yes, repo==manifest    | skip
//   yes          | yes      | yes, repo!=manifest    | update
//   yes          | yes      | no,  repo==manifest    | localModNoRepoChange (warn)
//   yes          | yes      | no,  repo!=manifest    | localModWithRepoUpdate (warn + overwrite)
//   yes          | no       | —                      | remove (if prune) else skip
//   no           | no       | —                      | ignore (non-managed)
//
func Plan(manifest *manifest.Manifest, repoSkills []repo.SkillDir, localSHAs map[string]string, prune bool) []Action
```

### Config (internal/config/config.go)

```go
type Config struct {
    RepoURL    string `yaml:"repo_url"`    // git repo URL (required)
    Branch     string `yaml:"branch"`      // default: "main"
    TargetDir  string `yaml:"target_dir"`  // default: "~/.config/opencode/skills"
}

const SlapDir    = "~/.config/slap"
const ConfigFile = SlapDir + "/config.yaml"
const ManifestFile = SlapDir + "/manifest.json"

func Load(path string) (*Config, error)           // expanded home dir, yaml.Unmarshal, os.ErrNotExist → error
func (c *Config) Save(path string) error           // mkdirAll + yaml.Marshal + atomic write
func (c *Config) ApplyFlagOverrides(repo, branch, targetDir string)
func ValidateRepoAccess(url, branch string) error  // go-git ls-remote or NewRemote + List
```

## Testing Strategy

Unit tests use `go test` (standard). Integration tests use build tag `//go:build integration` and are run via `go test -tags=integration ./...`.

| Layer | What to Test | Approach |
|-------|-------------|----------|
| **Unit** | `manifest.Manifest` marshal/unmarshal, Load/Save atomicity, Upsert/Remove business logic | Table-driven Go tests with temp dirs; test atomic write by simulating partial writes; test Load with corrupt JSON (expect backup + empty) |
| **Unit** | `sync.Plan` delta computation | Given manifest state + repo state + local SHAs → assert correct Action list; test all combos including local mods, missing folders, missing manifest, corrupt manifest |
| **Unit** | `repo.ComputeLocalTreeSHA` | Create temp dirs with known file structure, verify SHA is deterministic and changes when files change |
| **Unit** | `config` defaults | Verify default struct values match spec |
| **Integration** | `repo.Client.CloneShallow` + `ListSkillDirs` | Create in-memory go-git repo with skill dirs in test, seed with commits, clone via `PlainClone` to temp dir, verify `ListSkillDirs` returns correct names and tree SHAs |
| **Integration** | End-to-end sync via in-memory repo | Create in-memory source repo, clone to temp target dir, run sync Plan + Execute, verify files copied, manifest written correctly to slap dir |

No E2E tests (config.yaml: `e2e.available: false`). Linting via `go vet ./...` before every push.

## Migration / Rollout

No migration required — greenfield project. First release (`v0.1.0`) targets manual install via GitHub Releases. Homebrew formula is published only after the CLI is stable enough for regular use.

## Open Questions

- None. All decisions are resolved from specs and proposal.
