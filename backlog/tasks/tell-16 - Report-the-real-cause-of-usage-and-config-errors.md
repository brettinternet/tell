---
id: TELL-16
title: Report the real cause of usage and config errors
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 23:23'
updated_date: '2026-10-17 01:14'
labels: []
dependencies: []
documentation:
  - docs/design.md
  - docs/configuration.md
modified_files:
  - internal/cli/cli.go
  - internal/cli/notify.go
  - internal/cli/send.go
  - internal/cli/doctor.go
  - internal/config/config.go
  - docs/configuration.md
  - README.md
priority: high
type: bug
ordinal: 9000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: Tell hides the actual problem behind a generic pointer, so every mistake costs a second command or a source dive. Found in a CLI UX audit (built binary, isolated HOME):

- `tell notify --tittle x hi` and `tell notify -t success hi` print `tell: invalid notify invocation; run tell notify --help`; the parser already knows the unknown flag. `tell send` has the same gap.
- `tell notify --via nope hi` prints `could not load notification configuration; run tell config check`; send and doctor do the same. `tell config check` already prints the validated message (for example the allowed notify.via values), and AGENTS.md already treats those messages as safe to print.
- `tell --timeout 5 config check` prints `flag:--timeout: timeout must be ...`; the internal provenance label leaks into prose.
- `tell send --intent req hi` reports `intent="" outcome=usage reason=invalid_intent` without naming the bad value or the choices.
- `tell notfy hi` prints `unexpected argument "notfy"` because the root has an inspect action; no suggestion is offered.

Scope: error rendering in internal/cli, message wording for setting provenance in internal/config, docs examples that show the changed text, regression tests for exit codes and JSON shape.

Non-goals: changing exit codes, outcome/reason codes, or JSON field names (schema_version 1 contract); printing message bodies, tokens, or raw provider configuration; not-sent explanations (separate task); auto-correcting typos.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 An unknown or malformed flag on notify or send prints the parser message naming the flag plus a `tell <command> --help` hint, exits 2, and the --json error message carries the same text.
- [x] #2 notify, send, and doctor with invalid configuration print the same validation message as `tell config check` and exit 1, in human and --json output.
- [x] #3 Setting errors identify their source as the user typed it (`--timeout`, `TELL_TIMEOUT`, or file path and line) instead of internal labels such as `flag:--timeout`.
- [x] #4 Invalid --type and --intent values name the rejected value and list the accepted values.
- [x] #5 An unknown command such as `tell notfy hi` exits 2 with `unknown command "notfy"` and a did-you-mean suggestion when one is close; bare `tell` still only inspects.
- [x] #6 Tests cover the exit code and JSON shape for one notify usage error and one config error; README and docs/configuration.md examples match the new text; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Inspect parser, configuration validation, and existing error tests. 2. Preserve safe detailed errors and improve source/value/command wording without changing schema or exit codes. 3. Verify regression cases and task ci; review once, commit implementation, merge main, finalize task, and clean owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented detailed parser messages with command help hints, consistent config validation errors, readable flag/env error origins without changing inspection origins, rejected type/intent values, and root command suggestions. Preserved existing no-body-leak test: values identical to positional messages are omitted. Exposed the existing delivery secret scanner through a minimal diagnostic wrapper (additional boundary: internal/delivery/secrets.go); regression tests extend internal/cli/cli_test.go. Verification passed: go test ./internal/cli ./internal/config; task ci (history Gitleaks, govulncheck, formatting, vet, Staticcheck, unit, race, built-binary smoke); affected LSP diagnostics clean. Isolated built binary confirmed unknown flag exit 2/help hint, req intent details, timeout exit 1/source, notfy suggestion, invalid via JSON for notify/send/doctor, and bare inspection exit 0. Independent scoped review pending; next step resolve concrete findings then commit/merge and finalize.

Final verification: independent reviewer found two reproduced disclosure regressions (boolean parser values repeating a message; credentials in unknown YAML keys). Both fixed centrally; TestForwardedErrorsPreservePrivacy covers notify/send human+JSON and all config consumers. Focused tests, LSP, and task ci passed after fixes and again after rebasing onto current main. task check:staged passed. Implementation commit 83bcf2c fast-forward merged into main. Verified session receipt and Worktrunk listing before removal; owned fix/usage-errors worktree and branch removed, Herdr workspace wEX disappearance verified. No retained task worktrees, blockers, or next implementation steps. Shell safety guard interruption was resolved before committing; no checks bypassed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Detailed safe CLI errors now show parser causes/help, validation sources, rejected type/intent choices, and command suggestions while preserving exit codes and schema v1. Implemented and merged as 83bcf2c; focused tests, full task ci, staged checks, and isolated binary checks passed. One independent review completed; both validated privacy findings fixed and regression-tested. Owned worktree, branch, and Herdr workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->
