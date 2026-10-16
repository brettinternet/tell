---
id: TELL-11
title: Deliver notifications through a user-owned command
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 21:41'
updated_date: '2026-10-16 22:27'
labels: []
dependencies:
  - TELL-2
  - TELL-7
documentation:
  - docs/design.md
  - ../bump/docs/hooks.md
type: feature
ordinal: 2600
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: you want a thin layer you own for anything the built-in routes and Apprise do not cover (for example curl to ntfy, macOS say, or Hammerspoon) without shell templates or a plugin registry. A personal executable with a fixed contract gives that escape hatch and keeps the outcome table.

Context:
- The command is a routing field: personal config only, never shared tell.yaml (TELL-7). Like Noon hooks, it is an argv list, never a shell string.
- Strict YAML currently has no list-valued settings; this adds the first. Do not add an env or flag form that would need shell-style splitting.
- Contract: Tell runs the argv exactly; the prepared body arrives on the child's stdin, then EOF; title, notification type, and project arrive in environment variables. Exit 0 means accepted, 3 means not_sent, other non-zero means failed; timeout or cancellation after start means uncertain.

Scope: command route in internal/delivery, config schema for the argv list, local fixture executables, tests, docs with a worked example.

Non-goals: shell strings, templates or interpolation, multiple named commands, plugin discovery, commands from shared config.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 notify --via command runs the configured argv without a shell; a local fixture verifies exact argv, the body on stdin, and title, type, and project in the environment, and that Tell's own stdin is never inherited.
- [x] #2 Child exit 0, 3, and other non-zero map to accepted, not_sent, and failed; a missing executable is not_sent; timeout and cancellation are uncertain; shared tell.yaml rejects the command setting.
- [x] #3 Docs include a worked personal example and the exact contract; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add strict personal-only notify.command argv configuration, whole-list replacement and source tracking; omit raw argv from inspection and diagnostics. No env/flag argv form.
2. Extend the existing bounded subprocess runner for prepared-body stdin and TELL_TITLE/TELL_TYPE/TELL_PROJECT environment; implement command exit mapping without a shell.
3. Add focused config/CLI fixture regressions for trust, exact argv/stdin, outcomes and cancellation; document the complete contract and worked example.
4. Run focused checks and task ci, obtain one boundary-focused review, commit implementation, merge main, finalize task evidence, and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in .worktrees/feat-tell-11-command. Focused go test ./internal/config ./internal/cli ./internal/delivery passed. TestCommandNotify exercises exact literal argv, prepared stdin through EOF, metadata env overrides, untouched caller stdin, safe inspection/output, exit 0/3/1/5, missing executable and absent command config. TestNotifyInterruption now covers command timeout and cancellation after start. TestCommandConfig and strict YAML cases cover personal trust, whole-list replacement, provenance and invalid list inputs. task ci passed (gofmt, vet, Staticcheck, changelog, unit/race tests, build/smoke, Gitleaks, govulncheck: no reachable vulnerabilities). LSP diagnostics clean on edited Go files. README/configuration/design document the exact contract and worked personal say wrapper. One independent boundary-focused review is underway; commit/merge/cleanup pending.

Completed one independent reviewer pass: no validated findings. A 16KiB subprocess fixture with a helper retaining stdin returned accepted on macOS; preserved successful exit for Go ErrWaitDelay after bounded pipe cleanup, with cancellation still uncertain. Final task ci passed after that correction and again on integrated main. Implementation committed as 9b0354c (feat: deliver notifications through personal commands), rebased over concurrent documentation-only main changes and fast-forward merged. task check:staged passed before commit. No push. Verified session creation receipt, clean integrated worktree and idle shell pane; Worktrunk removed feat/tell-11-command worktree and branch, and Herdr workspace wDZ disappearance was verified. No remaining blocker or resumable implementation work.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented personal-only notify.command argv delivery with prepared stdin, explicit metadata environment, bounded execution and honest outcomes. Strict parsing and diagnostic secrecy covered by real local fixtures; task ci and staged checks pass, independent review has no validated findings. Merged implementation 9b0354c into main and removed owned worktree/branch/workspace; no push.
<!-- SECTION:FINAL_SUMMARY:END -->
