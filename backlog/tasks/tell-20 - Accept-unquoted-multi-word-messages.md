---
id: TELL-20
title: Accept unquoted multi-word messages
status: To Do
assignee: []
created_date: '2026-10-16 23:24'
labels: []
dependencies: []
documentation:
  - docs/design.md
modified_files:
  - internal/cli/notify.go
  - internal/cli/send.go
  - internal/cli/cli.go
  - README.md
  - docs/configuration.md
priority: medium
type: enhancement
ordinal: 13000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: `tell notify Build finished` and `tell notify hi there` fail with `notify requires one message argument or - for stdin` (exit 2), so every interactive message needs quotes. Joining positional words with spaces, as echo and say do, removes the most common usage error.

Constraint (docs/design.md, Scheduler hooks): stdin is read only when the caller passes - explicitly, so hook payloads stay with the hook. Keep that: - means stdin only when it is the sole argument; elsewhere it is a literal word.

Non-goals: reading stdin implicitly when no argument is given or stdin is not a TTY; shell quoting changes; any change to message preparation, redaction, or truncation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `tell notify Build finished` and `tell send Tests failed` deliver the arguments joined by single spaces; a single quoted argument is unchanged.
- [ ] #2 A lone `-` reads stdin; `-` among other words is literal text; no arguments is still a usage error with exit 2.
- [ ] #3 Help usage shows `MESSAGE... | -`; README and docs/configuration.md describe the rule; task ci passes.
<!-- AC:END -->
