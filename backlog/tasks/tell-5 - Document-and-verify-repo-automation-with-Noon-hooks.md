---
id: TELL-5
title: Document and verify repo automation with Noon hooks
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 23:40'
labels:
  - reviewed
dependencies:
  - TELL-2
  - TELL-3
  - TELL-8
  - TELL-12
documentation:
  - ../bump/docs/hooks.md
  - docs/design.md
type: docs
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: a common use of Tell is reporting recurring repo work from a scheduler. Noon is the worked example because it is the scheduler in use; Tell itself stays scheduler-neutral (docs/design.md, Scheduler hooks). Users need a small repo-owned hook script and example configuration, not a scheduler-specific protocol inside Tell.

Context (Noon lifecycle hooks, ../bump/docs/hooks.md):
- on_failure runs after failed or timed_out runs; on_success after succeeded. Hook failures never change job status.
- Hooks get the job environment (including NOON_PROJECT, NOON_JOB, NOON_RUN_ID) plus NOON_STATUS, NOON_EXIT_CODE, and NOON_LOG_FILE when a log exists. Stdin is a versioned JSON run summary.
- A hook gets 30 seconds, then TERM to its process group, 5 seconds, then KILL. The first 64 KiB of hook stdout/stderr is appended to the run log.
- Top-level hook defaults come only from user config or --config, never project noon.yaml; job-level hooks may live in noon.yaml.
- noon history NAME --limit N --json returns recent runs, so a hook can compare with the previous run without Tell keeping state. If Noon has shipped NOON-31 (NOON_PREVIOUS_STATUS in hook environments; ../bump/backlog), use that instead of querying history.
- A job failing every few minutes would otherwise notify every few minutes.
- Apprise can forward to third-party services and phones; job logs can contain secrets.
- Scheduled services do not inherit an interactive PATH, desktop session (D-Bus on Linux), Herdr session, or agent identity.

Scope: example hook script and Noon config snippet under examples/, docs, README links, integration fixtures.

Non-goals: a Noon integration package, changes to the sibling Noon checkout, parsing the JSON summary in Tell, state in Tell, resending notifications or rerunning jobs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 An isolated integration fixture runs the example hook with a sanitized service-style environment and a Noon-shaped JSON summary on stdin; a local fixture provider receives the expected destination and job, status, run, and project context, and the JSON summary is not consumed as the message.
- [x] #2 The example notifies only on status transitions (first failure after success, then recovery) using Noon's hook context or run history rather than Tell state, keeps hook outcome separate from job outcome, and documents the 30-second hook budget against Tell's timeout.
- [x] #3 Example messages carry job, status, run ID, and project, and point to noon logs; they never include log excerpts.
- [x] #4 Docs cover PATH, user config, and desktop session setup for services, verifying setup with tell doctor, explicit Herdr target selection, and how callers handle Tell's exit codes (best effort with || true; never rerun a job because a notification failed); task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add a POSIX repo hook and Noon job snippet using NOON_PREVIOUS_STATUS (NOON-31 merged as 2d3d376 during implementation); send literal transition context without logs or local state.
2. Document service setup, routing, unknown-history policy, budgets, and exit handling; link from README/design.
3. Exercise actual hook/Tell with isolated service environment and provider fixtures; task ci and one focused review.
4. Commit implementation, merge to main, record evidence, commit task completion, remove owned worktree and release claim.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation verification: sh -n examples/noon/notify.sh and mise exec -- task ci passed in tell-5-noon-hooks (2026-10-16). Smoke fixture checks first failure, timeout, repeated failure/success suppression, timeout recovery, unknown/pruned predecessor suppression, exact literal context/log pointer, destination and canonical project, and one provider attempt with hook success even when provider exits 7. JSON summary/log sentinel text is excluded by exact body assertion. No real services contacted. NOON-31 merged during work; replaced initial history-query prototype with POSIX shell and documented minimum Noon commit 2d3d376. Full CI includes vet, staticcheck, unit/race/smoke, Gitleaks and govulncheck. LSP supplemental checks unavailable (Go report unknown; Python server exited before prototype removal). Focused independent review pending.

Final acceptance/delivery: AC1/AC3 proven by TestNoonHook exact destination, metadata and body assertions under isolated HOME/XDG/PATH with Noon-shaped stdin and log sentinel. AC2 proven by transition table and failing-provider case; docs specify 10s Tell vs 30s Noon budget and no retries. AC4 service documentation covers PATH, HOME/XDG, desktop/D-Bus/macOS permissions, explicit Herdr session, doctor limitations, exits 0–5 and best-effort || true. task ci and task check:staged passed. Independent reviewer PASS: no validated findings; confirmed shipped Noon snapshot contract and context isolation. Implementation committed as f5d9bed and fast-forward merged to main. Owned worktree/branch tell-5-noon-hooks removed through Worktrunk; exact associated Herdr workspace wE5 disappearance verified. No push. No blocker or remaining item-scoped step.

Review: no findings. Hook transition table matches Noon's NOON_STATUS/NOON_PREVIOUS_STATUS contract (bump docs/hooks.md); message is literal argv with </dev/null, so JSON stdin and logs never reach Tell. Both examples pass current noon config check; smoke TestNoonHook passed in task ci.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added the POSIX Noon transition hook, job snippet, service/routing/budget documentation and isolated binary smoke coverage. Requires Noon NOON-31 (2d3d376+), sends no logs or JSON summary, and keeps notification failure best effort. Verified task ci, sh -n, staged checks and independent review. f5d9bed merged to main; owned worktree/branch/workspace cleaned; no push.
<!-- SECTION:FINAL_SUMMARY:END -->
