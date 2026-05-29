# Skills Status Specification

## Purpose

The `skills-cli status` command compares locally installed skills (from the manifest) against the configured repository's HEAD, reporting which skills are up-to-date, behind, new in the repo, or missing from the repo.

## Requirements

### Requirement: Status fetches repo HEAD without full clone

The system MUST fetch the latest commit sha from the configured repo without performing a full clone (e.g., `git ls-remote` or equivalent shallow fetch).

#### Scenario: Status fetches remote sha efficiently

- GIVEN the configured repo URL and branch
- WHEN the user runs `skills-cli status`
- THEN the system fetches the remote HEAD sha
- AND does not create any directory in `~/.config/opencode/skills/`

### Requirement: Status reports per-skill drift

For each skill in the manifest, the system MUST compare the recorded sha against the repo HEAD sha and classify the result.

#### Scenario: Up-to-date skill reported

- GIVEN a manifest skill's sha matches the repo HEAD sha for that skill
- WHEN the user runs `skills-cli status`
- THEN the skill is reported as "up-to-date"

#### Scenario: Behind skill reported

- GIVEN a manifest skill's sha differs from the repo HEAD sha for that skill
- WHEN the user runs `skills-cli status`
- THEN the skill is reported as "behind" with the available sha

### Requirement: Status detects new and missing skills

The system MUST compare the manifest entry set against the repo's skill list and report skills present only in one side.

#### Scenario: New skill available in repo

- GIVEN a skill folder exists in the repo but has no manifest entry
- WHEN the user runs `skills-cli status`
- THEN the skill is reported as "new (available for install)"

#### Scenario: Missing skill (in manifest, not in repo)

- GIVEN a manifest entry has no corresponding folder in the repo
- WHEN the user runs `skills-cli status`
- THEN the skill is reported as "missing (removed from repo)"

### Requirement: Status handles network errors gracefully

If the remote repo is unreachable, the system MUST report the failure and exit with a non-zero status. It MUST NOT modify any local files.

#### Scenario: Unreachable repo prints error

- GIVEN the configured repo URL is unreachable (no network, DNS failure)
- WHEN the user runs `skills-cli status`
- THEN the system prints an error message describing the failure
- AND exits with non-zero status
- AND no local files are created or modified

#### Scenario: No manifest prints guidance

- GIVEN no manifest file exists
- WHEN the user runs `skills-cli status`
- THEN the system prints "No skills installed. Run 'skills-cli sync' to install."
- AND exits with status code 0
