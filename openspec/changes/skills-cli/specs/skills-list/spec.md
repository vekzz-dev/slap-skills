# Skills List Specification

## Purpose

The `skills-cli list` command displays all skills managed by the manifest, showing their name, source repo, install date, and last synced sha. Skills installed outside the manifest (e.g., from other tools or manual setup) are NOT shown.

## Requirements

### Requirement: List shows manifest-tracked skills

The system MUST read `~/.config/opencode/skills/.opencode-skills-manifest.json` and display a table of all tracked skills with their metadata.

#### Scenario: Skills present in manifest are listed

- GIVEN the manifest contains entries for skills "foo" and "bar" with install dates and shas
- WHEN the user runs `skills-cli list`
- THEN output is a table with columns: Name, Source, Installed, Sha
- AND each manifest skill appears as one row

#### Scenario: Non-manifest skills are excluded

- GIVEN `~/.config/opencode/skills/sdd/` exists but is NOT in the manifest
- WHEN the user runs `skills-cli list`
- THEN the `sdd` skill is NOT shown in the output

#### Scenario: Empty manifest produces empty table

- GIVEN the manifest exists but contains no skills
- WHEN the user runs `skills-cli list`
- THEN output is an empty table with headers only

### Requirement: List handles missing manifest

If no manifest file exists, the system MUST report that no skills are installed via the sync tool, without erroring.

#### Scenario: No manifest shows empty state message

- GIVEN `~/.config/opencode/skills/.opencode-skills-manifest.json` does not exist
- WHEN the user runs `skills-cli list`
- THEN the system prints a message "No skills installed. Run 'skills-cli sync' to install."
- AND exits with status code 0

### Requirement: Output format is clean and parseable

The system SHOULD format the table using aligned columns. The system SHOULD support a `--json` flag to output as a JSON array.

#### Scenario: Table columns are aligned

- GIVEN manifest has skills with varying name lengths
- WHEN the user runs `skills-cli list`
- THEN all rows are aligned in fixed-width columns

#### Scenario: JSON flag outputs parseable array

- GIVEN manifest has two skills
- WHEN the user runs `skills-cli list --json`
- THEN output is a valid JSON array of skill objects with name, repo, installed_at, sha fields
