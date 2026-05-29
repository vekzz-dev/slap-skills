# Homebrew Formula Specification

## Purpose

A Homebrew formula that installs `slap` via `brew install`. The formula lives in the `vekzz-dev/homebrew-tap` repository and references prebuilt binaries published to GitHub Releases.

## Requirements

### Requirement: Formula installs from prebuilt binaries

The formula MUST define a `url` pointing to the `.tar.gz` archive from GitHub Releases for the user's platform, with a matching `sha256` checksum. It MUST support both ARM64 and AMD64 architectures for both macOS and Linux.

#### Scenario: Homebrew installs on macOS ARM64

- GIVEN the user runs `brew install vekzz-dev/tap/slap-skills` on Apple Silicon
- THEN the formula selects the `darwin_arm64` archive
- AND the binary is installed at the Homebrew prefix under `bin/slap`

#### Scenario: Homebrew installs on macOS Intel

- GIVEN the user runs `brew install vekzz-dev/tap/slap-skills` on Intel Mac
- THEN the formula selects the `darwin_amd64` archive
- AND `slap` is available on `PATH`

### Requirement: Formula is auto-updated by release pipeline

The formula MUST be structured so that GoReleaser's `brews` stanza can update the version, URL, and SHA on each new release. The formula class name MUST be `SlapSkills`.

#### Scenario: Formula field placeholders are updatable

- GIVEN a new release with version `v0.2.0`
- WHEN GoReleaser updates the formula
- THEN `version` is set to `0.2.0`
- AND `url` points to `https://github.com/vekzz-dev/slap-skills/releases/download/v0.2.0/slap-skills_0.2.0_darwin_arm64.tar.gz`
- AND `sha256` for each platform is updated correctly

### Requirement: Formula runs basic smoke test

The formula SHOULD include a `test` block that verifies the binary executes and returns a usage message.

#### Scenario: Test block validates binary works

- GIVEN the formula is installed
- WHEN `brew test slap-skills` runs
- THEN the system calls `slap --help`
- AND the exit code is 0

### Requirement: Formula references correct repository

The formula `homepage` MUST point to the slap-skills repository. The `url` MUST reference `vekzz-dev/slap-skills` releases.

#### Scenario: Homepage points to source repo

- GIVEN a user views the formula source
- THEN `homepage` is `https://github.com/vekzz-dev/slap-skills`
- AND `url` pattern uses the `github.com/vekzz-dev/slap-skills` release path
