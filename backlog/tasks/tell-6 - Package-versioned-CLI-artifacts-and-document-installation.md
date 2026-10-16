---
id: TELL-6
title: Package versioned CLI artifacts and document installation
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 20:47'
labels: []
dependencies:
  - TELL-2
references:
  - ../hum/.github/workflows/release.yaml
type: chore
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: once Tell can deliver notifications, consumers need reproducible binaries rather than building the repository on each machine.

Context:
- cmd/tell declares buildVersion, buildCommit, and buildTime for -X linker injection, but task cli:build injects nothing, so tell --version reports dev and unknown.
- ../hum/.github/workflows/release.yaml is the reference: tag-triggered, verifies the tag commit's main-branch CI run, builds CGO_ENABLED=0 -trimpath archives for linux and darwin on amd64 and arm64, and publishes checksums.txt.
- CI (.github/workflows/ci.yaml) runs task ci on Linux and macOS with SHA-pinned Actions.

Scope: Taskfile/.taskfiles packaging tasks, a release workflow, checksums, installation and version docs.

Non-goals: Windows, daemons, Homebrew publishing, changelog pushes from CI, tag pushes or release publication while implementing this task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A local packaging task produces Linux and macOS amd64/arm64 archives with linker-injected version, commit, and build time plus a checksum file that verifies; the host artifact's --version reports the injected metadata.
- [ ] #2 A release workflow verifies the exact tag commit passed CI before publishing, with minimal permissions and SHA-pinned Actions; no release is published during verification.
- [ ] #3 Installation and optional external-provider prerequisites are documented accurately; task ci and the local packaging checks pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
