---
id: TELL-18
title: Add short flags for common options
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:53'
labels:
  - reviewed
dependencies: []
documentation:
  - docs/configuration.md
modified_files:
  - internal/cli/cli.go
  - internal/cli/notify.go
  - README.md
  - docs/configuration.md
priority: high
type: enhancement
ordinal: 11000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: no Tell flag except -C/--project, -h, and -v has a short form, so interactive use means typing `--title`, `--type`, `--json`, and `--config` in full. Short forms should be predictable and must not collide.

Chosen letters (decided in the CLI UX audit):
- global: `-c` --config, `-j` --json (keep `-C` --project, `-v` --version, `-h` --help)
- notify: `-t` --title, `-n` --type (Apprise uses -t/-n for title and notification type; Tell wraps Apprise)
- send: `-i` --intent
- init: `-u` --user (and `-l` --local unless TELL-15 removes that layer)

Context: Run() re-parses argv after errors (jsonRequested, commandRequested) to decide JSON output and which command failed. Those helpers match long names, so a short `-j` or a value-taking `-c FILE` could be misread on the error path unless they learn the aliases.

Non-goals: a short form for --via (`-v` is version, and no other letter is predictable), --timeout, routing/provider flags, --to/--from (already short), or renaming long flags.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Each listed short flag works and is shown with its long form in --help.
- [x] #2 `tell -j notify --bogus hi` prints a JSON usage error, and `tell -c FILE notify hi` does not treat FILE as the command, matching their long-form behavior.
- [x] #3 README and docs/configuration.md show the short forms where they list flags; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add the selected aliases using existing CLI flag definitions and alias-aware error parsing. 2. Verify aliases/help and error regressions; update flag documentation. 3. Run task ci, review the diff, commit, merge locally, and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented aliases c/j/t/n/i/u. TELL-15 removed local initialization, so no l alias. Existing value-flag scanners already use flag.Names(); only JSON boolean detection required adjustment. Extended cli_test.go for error-protocol regressions (additional test boundary); notify.go needed no change. Verification: task ci passed (security, format, vet, Staticcheck, unit, race, smoke); LSP clean; isolated built-binary checks passed for every alias/help entry, captured title/type/body, config selection, send intent, user initialization, and JSON usage error. One scoped diff review found no defects. Ready for commit and local integration.

Committed implementation as 3dbaa53 and fast-forward merged into main. task check:staged passed before commit. Removed verified session-owned tell-18-short-flags worktree and branch via Worktrunk; its Herdr workspace wE0 was removed by the cleanup hook. No blockers or remaining work.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: @pi
created: 2026-10-17 01:53
---
Review: ACs verified against built binary (-c/-j/-t/-n/-i/-u work and show in --help; -j and -j=false select output mode on parse errors; -c FILE value is not taken as the command). Docs list short forms. No defects found.
---
<!-- COMMENTS:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added documented short aliases for common options and JSON error handling for -j, preserving existing long flags. Verified all aliases/help and delivery metadata with an isolated binary check, error-path regressions, and full task ci. Committed and merged locally as 3dbaa53; worktree cleanup complete. Nothing pushed.
<!-- SECTION:FINAL_SUMMARY:END -->
