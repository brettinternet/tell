---
id: TELL-16
title: Report the real cause of usage and config errors
status: To Do
assignee: []
created_date: '2026-10-16 23:23'
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
- [ ] #1 An unknown or malformed flag on notify or send prints the parser message naming the flag plus a `tell <command> --help` hint, exits 2, and the --json error message carries the same text.
- [ ] #2 notify, send, and doctor with invalid configuration print the same validation message as `tell config check` and exit 1, in human and --json output.
- [ ] #3 Setting errors identify their source as the user typed it (`--timeout`, `TELL_TIMEOUT`, or file path and line) instead of internal labels such as `flag:--timeout`.
- [ ] #4 Invalid --type and --intent values name the rejected value and list the accepted values.
- [ ] #5 An unknown command such as `tell notfy hi` exits 2 with `unknown command "notfy"` and a did-you-mean suggestion when one is close; bare `tell` still only inspects.
- [ ] #6 Tests cover the exit code and JSON shape for one notify usage error and one config error; README and docs/configuration.md examples match the new text; task ci passes.
<!-- AC:END -->
