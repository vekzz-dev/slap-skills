# Skills Sync Specification

## Purpose

The `slap sync` command pulls skills from the configured git repo and reconciles the local `~/.config/opencode/skills/` directory to match. It handles missing/corrupt manifest, detects locally-modified skills, adds new skills, updates changed ones, and optionally removes deleted ones — without touching skills from non-managed sources.

## Requirements

### Requirement: Sync reads config and manifest

The system MUST read repo URL and branch from `~/.config/slap/config.yaml`, then load the manifest from `~/.config/slap/manifest.json` to determine current state.

#### Scenario: Missing config is an error

- GIVEN `~/.config/slap/config.yaml` does not exist
- WHEN the user runs `slap sync`
- THEN the system prints "Slap is not configured. Run `slap init <repo-url>` first."
- AND exits with non-zero status

### Requirement: Sync adds new skills from repo

The system MUST shallow-clone the repo, compare repo skill folders against the manifest, and copy any new skills into the target directory.

#### Scenario: First-time sync with no manifest installs all repo skills

- GIVEN no manifest exists at `~/.config/slap/manifest.json`
- WHEN the user runs `slap sync`
- THEN all skill folders from the repo are copied into `~/.config/opencode/skills/`
- AND a manifest is created at `~/.config/slap/manifest.json` recording each skill name, tree SHA, and install date

#### Scenario: Re-run on unchanged repo is a no-op

- GIVEN the manifest matches the repo tree SHAs for all installed skills
- WHEN the user runs `slap sync`
- THEN no skill folders are modified
- AND the manifest is not rewritten

### Requirement: Sync updates changed skills

The system MUST compare each installed skill's recorded tree SHA (from manifest) against the repo's current tree SHA, and replace any skill folder whose SHA differs.

#### Scenario: Updated skill is replaced

- GIVEN a skill "foo" exists locally with manifest SHA `abc123`
- WHEN the repo contains skill "foo" with SHA `def456`
- THEN the local "foo" folder is replaced with the repo version
- AND the manifest is updated to SHA `def456`

#### Scenario: Partial update recovers on re-run

- GIVEN a previous sync was interrupted mid-update
- WHEN the user re-runs `slap sync`
- THEN the system resumes from current manifest state
- AND any partially-updated skill is re-synced from the repo

### Requirement: Sync with --prune removes deleted skills

If the `--prune` flag is provided, the system MUST remove any locally-installed skill folder that exists in the manifest but is absent from the repo.

#### Scenario: Prune removes deleted repo skill

- GIVEN skill "bar" exists in manifest but is absent from the repo
- WHEN the user runs `slap sync --prune`
- THEN the local "bar" folder is deleted
- AND "bar" is removed from the manifest

#### Scenario: Prune never removes non-managed skills

- GIVEN a folder `~/.config/opencode/skills/sdd-init` exists but is NOT in the manifest
- WHEN the user runs `slap sync --prune`
- THEN the `sdd-init` folder is untouched
- AND it is not added to the manifest

### Requirement: Manifest write is atomic and failure-resilient

The system MUST write the manifest by creating a temporary file in the same directory, then renaming it over the existing manifest. On any failure before rename completes, the original manifest SHALL remain untouched.

#### Scenario: Sync failure preserves prior manifest

- GIVEN a valid manifest exists
- WHEN the sync fails before rename completes (e.g., disk full, interrupted)
- THEN the original manifest file is preserved unmodified

#### Scenario: Next sync resumes from persisted manifest

- GIVEN a failed sync left the manifest unchanged
- WHEN the user runs `slap sync` again
- THEN the system uses the existing manifest to determine what needs updating

### Requirement: Sync recovers from missing manifest

If the manifest does not exist, the system MUST reconstruct it by scanning the target skills directory, matching folder names against the repo, and computing tree SHAs for each match. Folders that exist locally but NOT in the repo are assumed to be non-managed and are silently ignored.

#### Scenario: Missing manifest rebuilds from disk state

- GIVEN `~/.config/slap/manifest.json` does not exist
- AND skill "foo" and "bar" folders exist in both the repo and `~/.config/opencode/skills/`
- AND folder "baz" exists locally but not in the repo
- WHEN the user runs `slap sync`
- THEN "foo" and "bar" are added to the manifest with their current tree SHAs
- AND "baz" is NOT added to the manifest
- AND no files are copied (local state matches repo)

#### Scenario: Missing manifest with missing skills installs them

- GIVEN `~/.config/slap/manifest.json` does not exist
- AND skill "foo" exists in the repo but NOT in `~/.config/opencode/skills/`
- WHEN the user runs `slap sync`
- THEN "foo" is copied from the repo to `~/.config/opencode/skills/`
- AND "foo" is added to the manifest

### Requirement: Sync handles corrupt manifest

If the manifest file exists but is not valid JSON, the system MUST rename it to `manifest.json.bak`, warn the user, and run recovery (same as missing manifest rebuild).

#### Scenario: Corrupt manifest triggers recovery

- GIVEN `~/.config/slap/manifest.json` contains invalid JSON
- WHEN the user runs `slap sync`
- THEN the corrupt file is renamed to `manifest.json.bak`
- AND a warning is printed: "Previous manifest was corrupt, backed up to manifest.json.bak"
- AND recovery proceeds by scanning the skills directory against the repo

### Requirement: Sync detects locally-modified skills

Before updating a skill, the system MUST compare the current local tree SHA against the manifest SHA. If they differ AND the repo SHA also differs from the manifest, the system MUST warn the user and prefer the repo version (overwrite local changes).

#### Scenario: Locally modified skill is overwritten with warning

- GIVEN skill "foo" has manifest SHA `abc123`
- AND local skill "foo" has current SHA `999999` (user edited it)
- AND repo skill "foo" has SHA `def456`
- WHEN the user runs `slap sync`
- THEN a warning is printed: "Skill 'foo' was modified locally. Overwriting with repo version."
- AND "foo" is replaced with the repo version
- AND the manifest is updated to SHA `def456`

#### Scenario: Local-only modification without repo change is left untouched

- GIVEN skill "foo" has manifest SHA `abc123`
- AND local skill "foo" has current SHA `999999` (user edited it)
- AND repo skill "foo" has SHA `abc123` (no change in repo)
- WHEN the user runs `slap sync`
- THEN a warning is printed: "Skill 'foo' was modified locally. Repo version unchanged."
- AND the local version is NOT overwritten
- AND the manifest stays at SHA `abc123`
- AND no files are changed

### Requirement: Sync validates manifest consistency

After loading the manifest, the system MUST verify that every entry has a corresponding skill folder in the target directory. Missing folders are treated as if the skill was removed (removed from manifest, treated as "new" from repo perspective).

#### Scenario: Skill folder manually deleted is reinstalled

- GIVEN the manifest contains skill "foo"
- AND the folder `~/.config/opencode/skills/foo/` does not exist
- WHEN the user runs `slap sync`
- THEN "foo" is copied from the repo to the target directory
- AND the manifest entry is updated (install date reset)

#### Scenario: Non-managed folder is never touched

- GIVEN a folder `~/.config/opencode/skills/sdd-init` exists
- AND no manifest entry exists for `sdd-init`
- WHEN the user runs `slap sync`
- THEN the `sdd-init` folder is never read, compared, or modified
