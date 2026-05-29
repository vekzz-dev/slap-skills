# Slap Skills

[![Go Version](https://img.shields.io/github/go-mod/go-version/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills)
[![Go Report](https://goreportcard.com/badge/github.com/vekzz-dev/slap-skills)](https://goreportcard.com/report/github.com/vekzz-dev/slap-skills)
[![Release](https://img.shields.io/github/v/release/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/vekzz-dev/slap-skills/ci.yml?branch=main)](https://github.com/vekzz-dev/slap-skills/actions)
[![Homebrew](https://img.shields.io/badge/Homebrew-vekzz--dev%2Ftap%2Fslap--skills-orange)](https://github.com/vekzz-dev/homebrew-tap)
[![License](https://img.shields.io/badge/license-MIT-blue)](https://github.com/vekzz-dev/slap-skills/blob/main/LICENSE)

**Slap Skills** syncs opencode skills from any git repo to `~/.config/opencode/skills/` — without touching existing skills from other sources.

```bash
brew tap vekzz-dev/tap
brew install slap-skills

slap init https://github.com/user/your-skills
slap sync
```

---

## Quick start

```bash
# 1. Configure your skill repo
slap init https://github.com/user/your-skills

# 2. Install all skills
slap sync

# 3. See what's installed
slap list

# 4. Check for updates
slap status
```

---

## Installation

### Homebrew (recommended)

```bash
brew tap vekzz-dev/tap
brew install slap-skills
```

### Go install

```bash
go install github.com/vekzz-dev/slap-skills@latest
```

### Manual

Download the latest binary from [GitHub Releases](https://github.com/vekzz-dev/slap-skills/releases) for your platform.

---

## Commands

| Command | Description |
|---------|-------------|
| `slap init <repo-url>` | Configure a git repo as the skill source |
| `slap sync` | Install or update skills from the configured repo |
| `slap sync --prune` | Sync and remove local skills no longer in the repo |
| `slap list` | List installed skills |
| `slap list --json` | List installed skills as JSON |
| `slap status` | Show drift between local skills and the repo |

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo` | (from config) | Override repo URL |
| `--branch` | `main` | Git branch to sync from |
| `--target-dir` | `~/.config/opencode/skills` | Local skills directory |

---

## How it works

```
~/.config/slap/
├── config.yaml        ← Your repo URL, branch, target dir
└── manifest.json      ← Tracked skills with tree SHAs

~/.config/opencode/skills/
├── sdd-init/          ← Other skills (never touched)
├── your-skill-1/      ← Installed by Slap
└── your-skill-2/      ← Installed by Slap
```

Each sync:
1. **Pre-flight** — loads config, loads or repairs the manifest
2. **Clone** — shallow clones your repo to a temp directory
3. **Plan** — compares manifest state × repo state × local disk state
4. **Execute** — adds new skills, updates changed ones, optionally removes deleted ones
5. **Save** — writes the manifest atomically

### Robustness

| Scenario | Behavior |
|----------|----------|
| Manifest lost | Rebuilds by scanning the skills directory against the repo |
| Manifest corrupt | Backs up to `.json.bak` and rebuilds |
| Skill edited locally | Warns but preserves your changes if repo hasn't changed |
| Skill edited locally + repo updated | Warns and overwrites with repo version |
| Skill folder deleted manually | Reinstalls from repo |
| Non-managed skills | Never read, compared, or modified |

---

## Skill repo structure

Your skill repo should follow the opencode skill layout:

```
your-skills/
├── my-linter/
│   └── SKILL.md
├── my-framework/
│   ├── SKILL.md
│   └── references/
│       └── examples.md
└── ...
```

---

## Development

```bash
# Build
go build -o slap .

# Test
go test ./...

# Run
./slap --help
```

---

## License

MIT
