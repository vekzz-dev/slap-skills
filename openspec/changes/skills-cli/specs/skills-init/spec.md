# Skills Init Specification

## Purpose

The `slap init <repo-url>` command configures the tool by validating and persisting the git repo URL that the user wants to manage skills from. Stores config in `~/.config/slap/config.yaml`.

## Requirements

### Requirement: Init validates repo accessibility

The system MUST verify the given repo URL is reachable before persisting it. Validation SHALL use `git ls-remote` or go-git remote operations (no clone needed).

#### Scenario: Valid public repo is accepted

- GIVEN a reachable public git repo URL
- WHEN the user runs `slap init https://github.com/user/skills`
- THEN the system validates the repo is accessible
- AND prints a success message
- AND creates `~/.config/slap/config.yaml` with the given URL

#### Scenario: Unreachable repo is rejected

- GIVEN a URL that does not point to a reachable git repo
- WHEN the user runs `slap init https://example.com/nonexistent`
- THEN the system prints an error message
- AND does NOT create or modify `~/.config/slap/config.yaml`
- AND exits with a non-zero status code

### Requirement: Init creates config directory and file

The system MUST create `~/.config/slap/` if it does not exist, and write `config.yaml` with the repo URL, default branch (`main`), and default target directory.

#### Scenario: First init creates config

- GIVEN no `~/.config/slap/` directory exists
- WHEN the user runs `slap init https://github.com/user/skills`
- THEN the directory `~/.config/slap/` is created
- AND `~/.config/slap/config.yaml` contains the repo URL, branch `main`, and target dir `~/.config/opencode/skills`

#### Scenario: Re-init overwrites existing config

- GIVEN `~/.config/slap/config.yaml` exists with a previous repo URL
- WHEN the user runs `slap init https://github.com/other/repo`
- THEN the config is overwritten with the new URL
- AND the old config is not recoverable from the config file

### Requirement: Init supports optional branch flag

The system MUST accept a `--branch` flag to override the default branch (`main`).

#### Scenario: Custom branch is persisted

- GIVEN the user runs `slap init https://github.com/user/skills --branch develop`
- THEN `~/.config/slap/config.yaml` contains `branch: develop`

### Requirement: Init outputs confirmation

After successful init, the system MUST print a clear message telling the user what to do next.

#### Scenario: Success message with next step

- GIVEN a successful init
- WHEN the command completes
- THEN the output includes "Slap configured!" and suggests running `slap sync`
