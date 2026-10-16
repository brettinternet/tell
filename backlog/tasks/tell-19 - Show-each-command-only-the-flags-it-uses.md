---
id: TELL-19
title: Show each command only the flags it uses
status: To Do
assignee: []
created_date: '2026-10-16 23:24'
labels: []
dependencies:
  - TELL-14
documentation:
  - docs/configuration.md
modified_files:
  - internal/cli/cli.go
  - README.md
  - docs/configuration.md
  - docs/design.md
  - examples/noon/README.md
  - integration
priority: medium
type: enhancement
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: all nine setting flags are global, so `tell notify --help`, `tell send --help`, `tell init --help`, and even `tell config check --help` each list twelve global options, most irrelevant (send help lists --apprise-format; notify help lists --to and --from). Finding the one flag that matters is slow, and nothing signals that --via does nothing for send.

Decision: keep -C/--project, --config, --json, and --timeout global. Put notify routing flags (--via, --apprise-config, --apprise-tag, --apprise-format, --herdr-session) on notify; send routing flags (--send-via, --to, --from, --herdr-session) on send. doctor, config/config show/config check, and `tell mcp` (TELL-14) span both, so they accept every setting flag under the same names; docs already show `tell --via desktop doctor`. Keep flag names unchanged so each name means one thing everywhere. Tell has no published release, so moving flags after the command is acceptable before the compatibility contract starts.

Depends on TELL-14 because it adds `tell mcp` and its flags to the same command tree.

Non-goals: renaming flags or TELL_* variables, changing precedence or provenance labels, accepting command flags before the command name.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `tell --help` lists only global flags plus help/version; `tell notify --help` and `tell send --help` list only flags that affect that command; init help lists no setting flags.
- [ ] #2 doctor, config (all forms), and mcp accept every setting flag with unchanged names, values, precedence, and flag provenance.
- [ ] #3 Every example in README, docs, examples/noon, and smoke tests places command flags after the command and still runs; task ci passes.
<!-- AC:END -->
