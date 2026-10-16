---
id: TELL-22
title: Write help text with plain wording and examples
status: To Do
assignee: []
created_date: '2026-10-16 23:24'
labels: []
dependencies:
  - TELL-18
  - TELL-19
  - TELL-20
modified_files:
  - internal/cli/cli.go
  - README.md
priority: medium
type: docs
ordinal: 15000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: --help is the main reference at the terminal, and today it uses project jargon ("Delivery plumbing", "dispatch is not completion", "acceptance is not human acknowledgment" repeated in three command summaries), has no examples, lists commands as send, doctor, notify, config, init, and shows every value flag as `string` (`--timeout string`, `--via string`). Shell completion exists (`tell completion bash|zsh|fish`) but is undocumented.

Depends on the short-flag, flag-scoping, and multi-word message tasks so the examples are written once against the final flags.

Non-goals: man pages, colored help, completion of flag values, changing behavior.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Root help states in plain words what Tell does, lists commands as notify, send, doctor, config, init (then mcp if present), and states once that accepted means handed to the provider, not read.
- [ ] #2 notify, send, doctor, config, and init help each include two to four copy-pasteable examples.
- [ ] #3 Value flags show a meaningful placeholder (for example `--timeout duration`, `--type type`) and list accepted values and defaults.
- [ ] #4 README documents shell completion setup for bash, zsh, and fish, and each generated script loads in its shell without error; task ci passes.
<!-- AC:END -->
