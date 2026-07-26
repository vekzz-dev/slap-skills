# Multi-Source Sync Specification

## Purpose

Sync skills across all configured sources independently. Each source is cloned, reconciled, and updated in isolation — partial failure of one source MUST NOT block others. Re-running sync with no changes is a no-op.

## Requirements

### Requirement: Sync All Sources

The system SHALL iterate over all configured sources, clone or fetch each, reconcile per-source manifest entries, and update the global manifest.

#### Scenario: Sync all sources successfully

- GIVEN sources `work` and `community` are configured
- WHEN a user runs `slap sync`
- THEN each source is fetched and reconciled independently
- AND the manifest is updated with per-source entries
- AND the user sees a per-source summary

#### Scenario: Sync is a no-op when nothing changed

- GIVEN all sources are up-to-date and manifest matches
- WHEN a user runs `slap sync`
- THEN the system MUST output "Everything up-to-date"
- AND MUST NOT modify any files

### Requirement: Per-Source Error Isolation

The system SHALL handle each source independently — a failure in one source MUST NOT prevent other sources from syncing.

#### Scenario: One source fails, others succeed

- GIVEN source `work` is unreachable and source `community` is reachable
- WHEN the user runs `slap sync`
- THEN `work` reports an error with the failure reason
- AND `community` completes its sync successfully
- AND the manifest reflects only `community` entries

### Requirement: Manifest Reconciliation

The system SHALL reconcile the manifest against each source's available skills, adding new entries and removing entries for skills that no longer exist in the source (with user confirmation).

#### Scenario: New skill discovered during sync

- GIVEN source `work` added a new skill `bar` since last sync
- WHEN the user runs `slap sync`
- THEN `bar` is added to the manifest with `Source: "work"`
- AND skill files are installed

#### Scenario: Removed skill detected during sync

- GIVEN source `work` removed skill `old-skill` since last sync
- WHEN the user runs `slap sync`
- THEN the system prompts the user whether to remove `old-skill (work)`
- AND removes it from manifest only if confirmed
