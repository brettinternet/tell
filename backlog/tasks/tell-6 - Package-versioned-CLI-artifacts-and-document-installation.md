---
id: TELL-6
title: Package versioned CLI artifacts and document installation
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 22:50'
labels: []
dependencies:
  - TELL-4
references:
  - ../hum/.github/workflows/release.yaml
  - ../homebrew-tap/Formula/worklease.rb
  - ../worklease/.github/workflows/release.yml
modified_files:
  - .github/workflows/release.yaml
  - .taskfiles/cli.yaml
  - Taskfile.dist.yaml
  - scripts/
  - README.md
  - docs/development.md
  - docs/design.md
type: chore
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: once Tell can deliver notifications, consumers need reproducible binaries rather than building the repository on each machine. The first release also turns today's free-to-change interfaces into a compatibility contract.

Context:
- cmd/tell declares buildVersion, buildCommit, and buildTime for -X linker injection, but task cli:build injects nothing, so tell --version reports dev and unknown.
- ../hum/.github/workflows/release.yaml is the reference: tag-triggered, verifies the tag commit's main-branch CI run, builds CGO_ENABLED=0 -trimpath archives for linux and darwin on amd64 and arm64, and publishes checksums.txt.
- worklease names archives worklease-vX.Y.Z-OS-ARCH.tar.gz (OS macos or linux, ARCH arm64 or x64) containing bin/worklease; hum omits the v prefix and ships a flat archive. Follow worklease. The tap's scheduled workflow (../homebrew-tap) pulls new releases into formulas, so Tell's CI never touches the tap; adding the formula is a follow-up task. mise's github backend can install the same archives.
- Compatibility surfaces: config version 1, JSON schema_version 1, and the exit codes in docs/design.md. This task waits for TELL-4 because TELL-4 replaces the bootstrap hcom.to setting (TELL_HCOM_TO, --to); releasing first would break the config version 1 promise.
- CI (.github/workflows/ci.yaml) runs task ci on Linux and macOS with SHA-pinned Actions.

Scope: Taskfile/.taskfiles packaging tasks, a release workflow, checksums, installation, version, and compatibility docs.

Non-goals: Windows, daemons, updating the Homebrew tap from CI, changelog pushes from CI, tag pushes or release publication while implementing this task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A local packaging task produces Linux and macOS arm64/x64 archives named and laid out like worklease (tell-vX.Y.Z-OS-ARCH.tar.gz containing bin/tell) with linker-injected version, commit, and build time plus a checksum file that verifies; the host artifact's --version reports the injected metadata.
- [ ] #2 A release workflow verifies the exact tag commit passed CI before publishing, with minimal permissions and SHA-pinned Actions; no release is published during verification.
- [ ] #3 Docs cover installation (release archive, mise github backend, and the brettinternet tap as a pending follow-up), optional external-provider prerequisites, and the compatibility promise for config version 1, JSON schema_version 1, and exit codes; task ci and the local packaging checks pass.
- [ ] #4 Version-tag pushes publish the platform archives and checksums.txt to brettinternet/tell GitHub Releases (not only Actions artifacts), with release notes; adapt the relevant hum or worklease release workflow without importing their unrelated container, Windows, or changelog-push features.
- [ ] #5 The release workflow includes clean-environment mise installation smoke checks on Linux and macOS, x64 and arm64: mise exec github:brettinternet/tell@latest -- tell --version resolves the expected release using default asset selection, without asset_pattern, matching overrides, custom URLs, or a custom plugin. Documentation shows this plain GitHub-backend command. Before publication is authorized, validate archive layout and workflow checks locally; record the live install check as pending, never claim it ran or publish a release solely for verification.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
