# Source Management Specification

## Purpose

Manage skill source repositories via `slap source`. Users add, list, and remove sources with named aliases. Old single-repo configs auto-migrate to a `default` source.

## Requirements

### Requirement: Source Add

The system SHALL provide an interactive `slap source add` command that prompts for a Git URL and an alias, validates repository access, and writes a source config file under `~/.config/slap/sources/<alias>.yaml`.

#### Scenario: Add valid source

- GIVEN a user runs `slap source add`
- WHEN they provide a valid Git URL and a unique alias
- THEN the system validates the URL is reachable
- AND writes `<alias>.yaml` with `url`, `alias`, and `branch` fields
- AND confirms the source was added

#### Scenario: Add source with duplicate alias

- GIVEN a source alias `work` already exists
- WHEN a user tries to add another source with alias `work`
- THEN the system MUST reject with "alias already exists"
- AND the existing source file MUST NOT be modified

### Requirement: Source List

The system SHALL provide a `slap source list` command that displays all configured sources with their aliases, URLs, and branches.

#### Scenario: List configured sources

- GIVEN two sources are configured (`work` and `community`)
- WHEN a user runs `slap source list`
- THEN output shows both sources with alias, URL, and branch

#### Scenario: List with no sources

- GIVEN no sources are configured
- WHEN a user runs `slap source list`
- THEN the system MUST display "No sources configured"

### Requirement: Source Remove

The system SHALL provide a `slap source remove <alias>` command that prompts the user whether to uninstall that source's installed skills before deleting the source config.

#### Scenario: Remove source with skill uninstall

- GIVEN source `work` has 3 installed skills
- WHEN a user runs `slap source remove work` and confirms skill removal
- THEN all skills from `work` are uninstalled
- AND the `work` source config file is deleted

#### Scenario: Remove source without uninstall

- GIVEN source `work` has installed skills
- WHEN a user runs `slap source remove work` and declines skill removal
- THEN the source config file is deleted
- AND installed skills from `work` remain (orphaned)

#### Scenario: Remove nonexistent source

- GIVEN source `work` does not exist
- WHEN a user runs `slap source remove work`
- THEN the system MUST display "Source 'work' not found"

### Requirement: Auto-Migration from Single Repo

The system SHALL detect an old `config.yaml` with a single `repo_url` field on first `slap source` or `slap sync` command and auto-migrate it to a source `default`.

#### Scenario: Migrate old config on first command

- GIVEN `config.yaml` has `repo_url: https://github.com/user/skills.git` and no `sources/` dir
- WHEN the user runs `slap source list` or `slap sync`
- THEN the system backs up `config.yaml` to `config.yaml.bak`
- AND creates `sources/default.yaml` with the old URL
- AND rewrites `config.yaml` with the new format referencing source `default`
- AND updates the manifest to add `Source: "default"` on all existing entries
