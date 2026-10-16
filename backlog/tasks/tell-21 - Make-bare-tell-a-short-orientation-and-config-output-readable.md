---
id: TELL-21
title: Make bare tell a short orientation and config output readable
status: To Do
assignee: []
created_date: '2026-10-16 23:24'
labels: []
dependencies: []
documentation:
  - docs/configuration.md
modified_files:
  - internal/cli/cli.go
  - README.md
  - docs/configuration.md
priority: medium
type: enhancement
ordinal: 14000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: bare `tell` is the first thing a new user runs, and today it is identical to `tell config`: eleven %q-quoted keys (most empty), origins as quoted strings like ("default"), a quoted file list, and a disclaimer about provider acceptance. It does not say whether notify or send is set up or what to try next.

Scope: human output of bare `tell`, `tell config`, and `tell config show` in internal/cli.

Constraints: no-argument commands only inspect (AGENTS.md); bare tell must not run provider executables, so it suggests `tell doctor` instead of probing readiness.

Non-goals: --json output changes (schema_version 1), colors or TTY detection, new settings.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Bare `tell` prints a short summary: project, config files in use (or none), notify route with origin, send transport and recipient or "not configured", and next commands (`tell notify "hello"`, `tell doctor`, `tell init`, `tell --help`); it runs no subprocesses and exits 0, or exits 1 with the validation message on invalid config.
- [ ] #2 `tell config` and `tell config show` print aligned key, value, and origin columns; values are unquoted unless empty or whitespace-significant; origins read as default, file path, env NAME, or flag --name.
- [ ] #3 README quick tour and docs/configuration.md show the new output; task ci passes.
<!-- AC:END -->
