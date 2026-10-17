---
id: TELL-22
title: Write help text with plain wording and examples
status: Done
assignee:
  - '@brett'
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:56'
labels:
  - reviewed
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
- [x] #1 Root help states in plain words what Tell does, lists commands as notify, send, doctor, config, init (then mcp if present), and states once that accepted means handed to the provider, not read.
- [x] #2 notify, send, doctor, config, and init help each include two to four copy-pasteable examples.
- [x] #3 Value flags show a meaningful placeholder (for example `--timeout duration`, `--type type`) and list accepted values and defaults.
- [x] #4 README documents shell completion setup for bash, zsh, and fish, and each generated script loads in its shell without error; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Revise command summaries, ordering, examples, and value placeholders without changing behavior.
2. Document and load generated bash, zsh, and fish completions; inspect rendered help and run task ci.
3. Commit implementation, merge into main, record acceptance evidence, and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented and fast-forward merged f1e597fa4e5df435668d7ad4009e4c13a70d0d0d into main. Verification: go test ./internal/cli passed; rendered root and notify/send/doctor/config/init --help inspected (requested order, one acceptance explanation, 2–3 examples each, meaningful value placeholders and defaults). Generated bash completion sourced under bash --noprofile --norc and registered complete -p tell; zsh completion sourced under zsh -f with compinit -D; fish completion sourced under mise exec aqua:fish-shell/fish-shell@4.9.3 -- fish --no-config. All exited 0. task ci passed (vet, Staticcheck, unit, race, smoke, Gitleaks, govulncheck); task check:staged and git diff --check passed; LSP clean. One focused self-review found no concrete defects; no runtime behavior changed. Owned worktree and branch tell-22-help removed via Worktrunk; exact Herdr workspace wF7 disappeared. No blocker or remaining implementation step.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: @pi
created: 2026-10-17 01:56
---
Review: ACs verified (root wording/order, single acceptance statement, 2-3 runnable examples per command, placeholders/defaults; bash/zsh/fish completion scripts source cleanly). Fixed in 051663e: --type/--intent defaults rendered quoted ("info") unlike other defaults; now DefaultText. Low-severity follow-ups, not fixed: (1) urfave emits raw backtick placeholders in shell-completion descriptions (e.g. 'Notification `route`: ...') in zsh/fish; (2) notify example '--via stderr --title Tests' sets a title the stderr route never prints.
---
<!-- COMMENTS:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Clarified terminal help, added command examples and meaningful flag placeholders/defaults, and documented verified Bash/Zsh/Fish completion setup. CI passed; f1e597f merged into main and owned worktree cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->
