# Multi-Source Install Specification

## Purpose

Install skills from multiple configured sources with per-skill origin tracking. Skills from different sources with the same name coexist via source-alias disambiguation.

## Requirements

### Requirement: Install from Named Source

The system SHALL support `slap install --source <alias>` to install skills from a specific source, recording the source alias in the manifest's `SkillEntry.Source` field.

#### Scenario: Install skill from specific source

- GIVEN sources `work` and `community` are configured
- WHEN a user runs `slap install --source work foo`
- THEN the manifest records `{Name: "foo", Source: "work"}`
- AND the skill files are installed to the configured target

#### Scenario: Install same skill name from different sources

- GIVEN source `work` has skill `foo` and source `community` has skill `foo`
- WHEN a user installs both
- THEN both entries coexist in the manifest keyed by `work:foo` and `community:foo`
- AND both skill directories are installed

### Requirement: Source-Aware Display

The system SHALL display skills as `name (source-alias)` in all user-facing output (list, install confirmation, status).

#### Scenario: Display with source alias

- GIVEN skill `foo` from source `work` is installed
- WHEN a user runs `slap list`
- THEN the entry shows as `foo (work)`

#### Scenario: Display with --json

- GIVEN skills from multiple sources are installed
- WHEN a user runs `slap list --json`
- THEN each entry MUST include a `source` field with the source alias

### Requirement: Install All from Source

The system SHALL support `slap install <alias>` (without a skill name) to install all available skills from that source.

#### Scenario: Install all from source

- GIVEN source `work` has 5 available skills
- WHEN a user runs `slap install work`
- THEN all 5 skills are installed with `Source: "work"` in the manifest
