---
id: TELL-21
title: Make bare tell a short orientation and config output readable
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:56'
labels:
  - reviewed
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
- [x] #1 Bare `tell` prints a short summary: project, config files in use (or none), notify route with origin, send transport and recipient or "not configured", and next commands (`tell notify "hello"`, `tell doctor`, `tell init`, `tell --help`); it runs no subprocesses and exits 0, or exits 1 with the validation message on invalid config.
- [x] #2 `tell config` and `tell config show` print aligned key, value, and origin columns; values are unquoted unless empty or whitespace-significant; origins read as default, file path, env NAME, or flag --name.
- [x] #3 README quick tour and docs/configuration.md show the new output; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Separate safe bare-command orientation from aligned detailed config output, preserving JSON. 2. Update README and configuration examples; verify formatting and no-delivery behavior. 3. Run task ci, review once, commit, merge locally, finalize task, and clean owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Verification: mise exec -- go test ./internal/cli and mise exec -- task ci passed (format, vet, Staticcheck, unit, race, smoke, Gitleaks, govulncheck). LSP diagnostics clean. Direct built-binary checks in an isolated temporary HOME/PATH verified default and configured summaries, no invocation of marker-writing command/hcom fixtures, config/show equivalence, empty/padded quoting and default/file/env/flag origins, unchanged schema_version 1 JSON, and invalid-config exit 1 with validation message. git diff --check passed. One scoped general review found no concrete defects. Implementation stays within the three specified files; no new tests needed for presentation-only changes.

Delivery: ca3c899 (feat(cli): simplify orientation and config output) committed after task check:staged and fast-forward merged into main. Owned tell-21 worktree and branch removed using Worktrunk; matching Herdr workspace wF6 disappearance verified. No remaining blocker or resumable implementation step; no push performed.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: @pi
created: 2026-10-17 01:56
---
Review: no defects. Verified with built binary in isolated HOME: bare tell summary (none/configured files, notify origin, send 'not configured' vs 'herdr → x y'), exit 1 with file:line validation message on invalid config, no provider calls; config/config show aligned table with quoted empty/double-space values and origins default/path/env NAME/flag --name. README and docs/configuration.md match output. Note: Next line always suggests tell init even when config exists (acceptable per AC).
---
<!-- COMMENTS:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Bare tell now offers a safe short orientation and next commands; config/show display aligned values and readable origins. README and configuration examples updated. Verified task ci, direct isolated binary checks, clean LSP diagnostics, and scoped review. Implemented and merged locally as ca3c899; worktree/workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->
