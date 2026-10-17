---
id: TELL-20
title: Accept unquoted multi-word messages
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:53'
labels:
  - reviewed
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
- [x] #1 `tell notify Build finished` and `tell send Tests failed` deliver the arguments joined by single spaces; a single quoted argument is unchanged.
- [x] #2 A lone `-` reads stdin; `-` among other words is literal text; no arguments is still a usage error with exit 2.
- [x] #3 Help usage shows `MESSAGE... | -`; README and docs/configuration.md describe the rule; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Join notify/send positional arguments with single spaces; read stdin only for a sole dash and retain missing-message usage errors.
2. Update usage and message-input documentation; extend existing input-isolation tests for multi-word and mixed-dash parsing.
3. Run focused tests, direct help checks, and task ci; review the bounded diff, commit, merge main, record evidence, and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented multi-word joining and sole-dash stdin handling. Regression tests exposed urfave v3.13 dropping arguments after a bare dash; Run now protects a message dash with an end-of-options marker, excluding flag values. Existing input-isolation tests cover joining, quoted spacing, leading/middle dash, sole-dash stdin, missing-input exit 2 and no dispatch; the obsolete extra-argument rejection is replaced by acceptance coverage. Validation: mise exec go -- go test ./internal/cli passed; mise exec -- task ci passed (security, formatting, vet, Staticcheck, tests, race, binary smoke); LSP diagnostics clean for all five Go files; git diff --check passed. Built bin/tell notify --help and send --help both show MESSAGE... | -. README and configuration docs updated. One bounded general diff review completed: no remaining item-scoped defects. Implementation changes also include the two existing input test files to verify the parser boundary. Next: commit implementation, fast-forward main, finalize task, remove owned checkout.

Delivery: implementation commit 1ae6e53736658f0108bef40b1498b56bd054fb4c passed task check:staged and was fast-forward merged to main. Worktrunk removed the verified session-owned tell-20-multiword checkout and branch; Herdr workspace wF5 removal verified. No retained checkout, blocker, or remaining implementation step. No push performed.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: @pi
created: 2026-10-17 01:53
---
Review: joining, quoted spacing, mid-dash literal, and missing-message exit 2 verified. Defect fixed in 2c81be0: preserveMessageDash inserted -- before the dash, so 'tell notify - --title X' sent the literal text '- --title X' and ignored stdin, unlike 'tell notify hi --title X'. Command flags are now moved ahead of the end-of-options marker; regression added to TestNotifyInputIsolation. Also widened the flaky herdr send timeout test (150ms to 1s) that failed under -race when probes exceeded the budget.
---
<!-- COMMENTS:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
notify and send now join message words with single spaces while preserving quoted spacing and reading stdin only for a sole dash. Help and docs describe MESSAGE... | -. Verified by expanded input-isolation tests, full task ci, clean LSP diagnostics, built-binary help, and bounded diff review. Implementation 1ae6e53 merged to main; owned worktree, branch, and workspace removed.
<!-- SECTION:FINAL_SUMMARY:END -->
