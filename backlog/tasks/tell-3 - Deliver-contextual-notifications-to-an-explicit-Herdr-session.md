---
id: TELL-3
title: Deliver contextual notifications to an explicit Herdr session
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 20:28'
updated_date: '2026-10-16 22:32'
labels: []
dependencies:
  - TELL-2
documentation:
  - docs/design.md
type: feature
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: repo jobs need contextual feedback in the Herdr session a person is watching, without relying on the shell that launched the job. Scheduled services do not inherit a Herdr session, so the target must be explicit.

Context:
- Herdr CLI (0.9.x): herdr notification show <TITLE> [--body TEXT] [--sound none|done|request]. The global --session <name> option selects a named session, but on launch it also creates sessions. Panes export HERDR_ENV, HERDR_SOCKET_PATH, HERDR_PANE_ID, and related variables that point at the current session. Confirm how --session and HERDR_SOCKET_PATH interact before relying on either.
- Reuse TELL-2's notify contract, subprocess helper, and JSON result. Message preparation (TELL-9) and secret redaction (TELL-10) run before every route; do not reimplement them per route.

Scope: the Herdr route in internal/delivery, CLI/config changes only if exercised, local fixture executables, tests, docs, and examples.

Non-goals: pane, tab, or workspace control; agent prompting; starting or creating Herdr sessions or servers; guessing a session; falling back to desktop or another route.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 task test exercises notify --via herdr through a local fixture executable and verifies that the title, body, and configured herdr.session reach Herdr as separate literal argv elements.
- [x] #2 A missing herdr.session or herdr executable returns not_sent before anything runs; inherited HERDR_* variables that point at another session never change the target; an unreachable session fails visibly per docs/design.md outcomes without creating or starting a session or choosing another destination.
- [x] #3 task ci passes; Herdr delivery keeps TELL-2's timeout and diagnostics contract (no message bodies or provider output in results), and docs state that a posted notification is not human acknowledgment.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Verify Herdr 0.9.5 session/socket and notification parsing against upstream source. 2. Add explicit-session delivery using shared process handling, isolate inherited Herdr routing, and fail closed on provider-reserved message tokens. 3. Exercise argv, missing target/tool, failures, timeout, and isolation with local fixtures; document behavior. 4. Run task ci and independent target-safety review, commit implementation, merge main, finalize backlog and remove owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented in .worktrees/tell-3-herdr. Verification: mise exec -- go test ./internal/cli ./internal/delivery passed; mise exec -- task ci passed (unit, race, smoke, vet, Staticcheck, formatting, Gitleaks, govulncheck). All affected Go LSP diagnostics clean. AC1: local fixture verifies configured session and prepared title/body as separate NUL-recorded argv. AC2: fixtures verify no execution without target, missing tool, all inherited HERDR_* stripped, no execution during inspection, reserved global-option payload rejection. Direct built Tell invocation against installed Herdr 0.9.5 with isolated temporary HOME/XDG and wrong inherited socket/session returned exit 4 failed/provider_failure for unreachable session; no Herdr/session directory created and no payload diagnostics. Upstream v0.9.5 session.rs/main.rs/cli notification and remote parsers confirm explicit-session precedence and connect-only dispatch. AC3: interruption fixture covers timeout and cancellation, provider-output privacy and nonzero exits; documentation explicitly distinguishes posting from acknowledgment. Independent target-safety review pending; next: resolve findings, commit/merge, finalize and clean owned checkout.

Independent reviewer completed with no validated findings after tracing upstream session/remote/machine/help parsing and notification dispatch. Implementation committed as 3fbc26dc7bbf81b2b7e80278e5abf5b5a45c6316 after task check:staged, fast-forward merged into main. Owned tell-3-herdr worktree and branch removed with Worktrunk; associated Herdr workspace disappearance verified. No push, no remaining blocker or resumable implementation step.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented explicit existing-session Herdr notifications with inherited context isolation, reserved-payload safety checks, shared bounded subprocess outcomes, and documented limitations. Verified task ci, clean LSP diagnostics, local argv/failure/interruption fixtures, real Herdr 0.9.5 unreachable-session behavior, and independent review. Commit 3fbc26d merged into main; owned worktree/branch/workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->
