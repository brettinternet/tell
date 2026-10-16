---
id: TELL-6
title: Package versioned CLI artifacts and document installation
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 20:29'
labels: []
dependencies:
  - TELL-2
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Once Tell can deliver notifications, consumers need reproducible binaries rather than building the repository for each machine. Scope: build/release tooling, GitHub workflow, checksums, and installation/version documentation. Follow sibling hum release conventions but keep only supported Linux/macOS CLI targets. Do not copy daemon, Windows, Homebrew publishing, or generated-changelog push machinery without an exercised requirement. Implementing this task does not authorize a tag push or release publication.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Local build commands produce Linux and macOS artifacts with linker-injected version/commit/build time and checksums, and the host artifact reports the injected metadata.
- [ ] #2 A release workflow verifies the exact tag commit passed CI before publishing, with minimal permissions and pinned Actions; no release is published during verification.
- [ ] #3 Installation and external-provider prerequisites are documented accurately; task ci and the local packaging checks pass.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
