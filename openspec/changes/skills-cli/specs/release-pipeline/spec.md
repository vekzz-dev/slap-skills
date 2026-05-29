# Release Pipeline Specification

## Purpose

A GoReleaser configuration and GitHub Actions CI workflow that builds the skills-cli binary for multiple platforms, creates GitHub Releases with signed checksums, and triggers the Homebrew formula update.

## Requirements

### Requirement: GoReleaser builds cross-platform binaries

The `.goreleaser.yaml` configuration MUST define build targets for Linux x86-64, Linux ARM64, macOS x86-64 (amd64), and macOS ARM64. Each binary MUST be statically linked where possible.

#### Scenario: Snapshot produces all targets

- GIVEN the repository has a valid `go.mod` and Go source
- WHEN the user runs `goreleaser --snapshot --clean`
- THEN binaries are produced for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- AND each binary is placed in `dist/` with OS/arch in the filename

### Requirement: Release artifacts include tarballs and checksums

Each release MUST produce a `.tar.gz` archive per platform and a `checksums.txt` file containing SHA-256 hashes of all archives.

#### Scenario: Release artifacts are complete

- GIVEN a tagged version (e.g., `v1.0.0`)
- WHEN GoReleaser runs
- THEN four `.tar.gz` archives are created (one per target)
- AND a `checksums.txt` with SHA-256 hashes for each archive is produced

### Requirement: GitHub Actions triggers on version tags

A `.github/workflows/release.yml` workflow MUST run GoReleaser when a tag matching `v*` is pushed. It MUST set up Go 1.x, checkout the repo, and invoke goreleaser.

#### Scenario: CI builds on semver tag push

- GIVEN a tag `v1.2.3` is pushed
- WHEN the release workflow runs
- THEN GoReleaser produces the release artifacts
- AND a GitHub Release named `v1.2.3` is created with archives and checksums attached

#### Scenario: Non-version tags are ignored

- GIVEN a tag `nightly-build` is pushed (no `v` prefix)
- WHEN the workflow triggers
- THEN the release job is skipped

### Requirement: Homebrew formula is auto-updated

The GoReleaser configuration SHOULD include a `brews` stanza that updates the Homebrew formula in `vekzz-dev/homebrew-tap` with the new version, archive URL, and SHA.

#### Scenario: Release updates brew formula

- GIVEN GoReleaser completes a tagged release
- THEN the formula file in `vekzz-dev/homebrew-tap` is updated with the new version and SHA-256
- AND a pull request or commit is made to that tap repository
