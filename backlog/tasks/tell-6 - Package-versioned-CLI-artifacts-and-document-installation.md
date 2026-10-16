---
id: TELL-6
title: Package versioned CLI artifacts and document installation
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 21:55'
labels: []
dependencies:
  - TELL-2
references:
  - ../hum/.github/workflows/release.yaml
  - ../homebrew-tap/Formula/worklease.rb
type: chore
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: once Tell can deliver notifications, consumers need reproducible binaries rather than building the repository on each machine. The first release also turns today's free-to-change interfaces into a compatibility contract.

Context:
- cmd/tell declares buildVersion, buildCommit, and buildTime for -X linker injection, but task cli:build injects nothing, so tell --version reports dev and unknown.
- ../hum/.github/workflows/release.yaml is the reference: tag-triggered, verifies the tag commit's main-branch CI run, builds CGO_ENABLED=0 -trimpath archives for linux and darwin on amd64 and arm64, and publishes checksums.txt.
- hum and worklease name archives NAME-vX.Y.Z-OS-ARCH.tar.gz with OS macos or linux and ARCH arm64 or x64. Their formulas live in ../homebrew-tap and are updated there after a release; mise's github backend can install the same archives.
- Compatibility surfaces: config version 1, JSON schema_version 1, and the exit codes in docs/design.md.
- CI (.github/workflows/ci.yaml) runs task ci on Linux and macOS with SHA-pinned Actions.

Scope: Taskfile/.taskfiles packaging tasks, a release workflow, checksums, installation, version, and compatibility docs.

Non-goals: Windows, daemons, updating the Homebrew tap from CI, changelog pushes from CI, tag pushes or release publication while implementing this task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A local packaging task produces Linux and macOS arm64/x64 archives named like hum and worklease (tell-vX.Y.Z-OS-ARCH.tar.gz) with linker-injected version, commit, and build time plus a checksum file that verifies; the host artifact's --version reports the injected metadata.
- [ ] #2 A release workflow verifies the exact tag commit passed CI before publishing, with minimal permissions and SHA-pinned Actions; no release is published during verification.
- [ ] #3 Docs cover installation (release archive, mise github backend, and the brettinternet tap once its formula exists), optional external-provider prerequisites, and the compatibility promise for config version 1, JSON schema_version 1, and exit codes; task ci and the local packaging checks pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
