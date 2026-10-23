---
id: TELL-20
title: Accept unquoted multi-word messages
status: done
type: enhancement
priority: medium
assignees: ['@pi']
labels: [reviewed]
rank: a969696969696968c
refs: [docs/design.md]
files: [internal/cli/notify.go, internal/cli/send.go, internal/cli/cli.go, README.md, docs/configuration.md]
created: 2026-10-16T23:24:00Z
updated: 2026-10-17T01:53:00Z
---

Why: `tell notify Build finished` and `tell notify hi there` fail with `notify requires one message argument or - for stdin` (exit 2), so every interactive message needs quotes. Joining positional words with spaces, as echo and say do, removes the most common usage error.

Constraint (docs/design.md, Scheduler hooks): stdin is read only when the caller passes - explicitly, so hook payloads stay with the hook. Keep that: - means stdin only when it is the sole argument; elsewhere it is a literal word.

Non-goals: reading stdin implicitly when no argument is given or stdin is not a TTY; shell quoting changes; any change to message preparation, redaction, or truncation.

## Acceptance criteria

- [x] `tell notify Build finished` and `tell send Tests failed` deliver the arguments joined by single spaces; a single quoted argument is unchanged.
- [x] A lone `-` reads stdin; `-` among other words is literal text; no arguments is still a usage error with exit 2.
- [x] Help usage shows `MESSAGE... | -`; README and docs/configuration.md describe the rule; task ci passes.

## Plan

1. Join notify/send positional arguments with single spaces; read stdin only for a sole dash and retain missing-message usage errors.
2. Update usage and message-input documentation; extend existing input-isolation tests for multi-word and mixed-dash parsing.
3. Run focused tests, direct help checks, and task ci; review the bounded diff, commit, merge main, record evidence, and remove the owned worktree.

## Notes

Implemented multi-word joining and sole-dash stdin handling. Regression tests exposed urfave v3.13 dropping arguments after a bare dash; Run now protects a message dash with an end-of-options marker, excluding flag values. Existing input-isolation tests cover joining, quoted spacing, leading/middle dash, sole-dash stdin, missing-input exit 2 and no dispatch; the obsolete extra-argument rejection is replaced by acceptance coverage. Validation: mise exec go -- go test ./internal/cli passed; mise exec -- task ci passed (security, formatting, vet, Staticcheck, tests, race, binary smoke); LSP diagnostics clean for all five Go files; git diff --check passed. Built bin/tell notify --help and send --help both show MESSAGE... | -. README and configuration docs updated. One bounded general diff review completed: no remaining item-scoped defects. Implementation changes also include the two existing input test files to verify the parser boundary. Next: commit implementation, fast-forward main, finalize task, remove owned checkout.

Delivery: implementation commit 1ae6e53736658f0108bef40b1498b56bd054fb4c passed task check:staged and was fast-forward merged to main. Worktrunk removed the verified session-owned tell-20-multiword checkout and branch; Herdr workspace wF5 removal verified. No retained checkout, blocker, or remaining implementation step. No push performed.

## Summary

notify and send now join message words with single spaces while preserving quoted spacing and reading stdin only for a sole dash. Help and docs describe MESSAGE... | -. Verified by expanded input-isolation tests, full task ci, clean LSP diagnostics, built-binary help, and bounded diff review. Implementation 1ae6e53 merged to main; owned worktree, branch, and workspace removed.

## Comments

### 2026-10-17T01:53:00Z @pi

Review: joining, quoted spacing, mid-dash literal, and missing-message exit 2 verified. Defect fixed in 2c81be0: preserveMessageDash inserted -- before the dash, so 'tell notify - --title X' sent the literal text '- --title X' and ignored stdin, unlike 'tell notify hi --title X'. Command flags are now moved ahead of the end-of-options marker; regression added to TestNotifyInputIsolation. Also widened the flaky herdr send timeout test (150ms to 1s) that failed under -race when probes exceeded the budget.
