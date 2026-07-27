# Slap Skills

[![Go Version](https://img.shields.io/github/go-mod/go-version/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills)
[![Go Report](https://goreportcard.com/badge/github.com/vekzz-dev/slap-skills)](https://goreportcard.com/report/github.com/vekzz-dev/slap-skills)
[![Release](https://img.shields.io/github/v/release/vekzz-dev/slap-skills)](https://github.com/vekzz-dev/slap-skills/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/vekzz-dev/slap-skills/ci.yml?branch=main)](https://github.com/vekzz-dev/slap-skills/actions)
[![Homebrew](https://img.shields.io/badge/Homebrew-vekzz--dev%2Ftap%2Fslap--skills-orange)](https://github.com/vekzz-dev/homebrew-tap)
[![License](https://img.shields.io/badge/license-MIT-blue)](https://github.com/vekzz-dev/slap-skills/blob/main/LICENSE)

[🇪🇸 Español](README.es.md)

**Slap Skills** is for those who don't trust random skills from the internet. Own your workflow. Keep your AI agent skills in your own git repos — public or private — and sync them to your machine with one command. No npx, no registries, no third-party dependencies. Just git and a `slap sync`.

```bash
brew tap vekzz-dev/tap
brew install slap-skills

slap init
slap source add --alias work https://github.com/user/work-skills
slap sync
```

---

## Quick start

```bash
# 1. Run the setup wizard (interactive)
slap init

# 2. Or add a source directly
slap source add

# 3. Install skills (choose which ones, or --all)
slap install --all

# 4. Keep them updated
slap sync

# 5. See what's installed
slap list

# 6. Check for updates
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
| `slap init` | One-time setup wizard (adds your first source) |
| `slap source add` | Interactive: add a new skill source with alias |
| `slap source list` | List all configured sources |
| `slap source remove <alias>` | Remove a source (prompts to uninstall its skills) |
| `slap install` | Select which skills to install from all sources |
| `slap install --all` | Install all skills without prompting |
| `slap install --source <alias>` | Install only from a specific source |
| `slap sync` | Update installed skills from all sources |
| `slap sync --prune` | Sync and remove skills no longer in any source |
| `slap list` | List installed skills (shows `name (source)`) |
| `slap list --json` | List installed skills as JSON (includes `source` field) |
| `slap status` | Show drift per source between local and remote |
| `slap remove <skill>` | Remove a skill by name |
| `slap remove <source>:<skill>` | Remove a skill from a specific source |
| `slap remove --all` | Remove all installed skills and clean the manifest |
| `slap version` | Print the current version |

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--branch` | `main` | Git branch to sync from |
| `--target-dir` | `~/.config/opencode/skills` | Local skills directory |

---

## How it works

```
~/.config/slap/
├── config.yaml            ← Sources list, target dir
├── sources/
│   ├── default.yaml       ← Source config (alias, url, branch)
│   ├── work.yaml
│   └── community.yaml
└── manifest.json          ← Tracked skills with source and tree SHAs

~/.config/opencode/skills/
├── sdd-init/              ← Other skills (never touched)
├── your-skill-1/          ← Installed by Slap
└── your-skill-2/          ← Installed by Slap
```

Each sync:
1. **Pre-flight** — loads config and sources, loads or repairs the manifest
2. **Per-source clone** — shallow clones each source repo to an isolated temp dir
3. **Per-source plan** — compares manifest state × repo state × local disk state for each source
4. **Execute** — adds new skills, updates changed ones, optionally removes deleted ones
5. **Save** — writes the manifest atomically once

### Display

Skills are always shown with their source alias: `skill-name (source-alias)`. This makes it clear where each skill comes from, especially when the same skill name exists in multiple sources.

### Robustness

| Scenario | Behavior |
|----------|----------|
| Manifest lost | Rebuilds by scanning the skills directory against sources |
| Manifest corrupt | Backs up to `.json.bak` and rebuilds |
| Skill edited locally | Warns but preserves your changes if source hasn't changed |
| Skill edited locally + source updated | Warns and overwrites with source version |
| Skill folder deleted manually | Reinstalls from source |
| One source unreachable | Other sources still sync (per-source error isolation) |
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

## Roadmap

Slap Skills currently targets **opencode**, but the vision is bigger. Future plans include supporting other AI coding agents — Claude Code, Cursor, Copilot, and any agent that loads local skill/instruction files — so you can manage **all your agent skills from one place**.

---

## License

MIT
