---
id: TELL-5
title: Document and verify repo automation with Noon hooks
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 21:30'
labels: []
dependencies:
  - TELL-2
  - TELL-3
  - TELL-8
documentation:
  - ../bump/docs/hooks.md
  - docs/design.md
type: docs
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: the primary use of Tell is reporting recurring repo work supervised by Noon. Users need a small repo-owned hook script and example configuration, not a Noon-specific protocol inside Tell.

Context (Noon lifecycle hooks, ../bump/docs/hooks.md):
- on_failure runs after failed or timed_out runs; on_success after succeeded. Hook failures never change job status.
- Hooks get the job environment (including NOON_PROJECT, NOON_JOB, NOON_RUN_ID) plus NOON_STATUS, NOON_EXIT_CODE, and NOON_LOG_FILE when a log exists. Stdin is a versioned JSON run summary.
- A hook gets 30 seconds, then TERM to its process group, 5 seconds, then KILL. The first 64 KiB of hook stdout/stderr is appended to the run log.
- Top-level hook defaults come only from user config or --config, never project noon.yaml; job-level hooks may live in noon.yaml.
- noon history NAME --limit N --json returns recent runs, so a hook can compare with the previous run without Tell keeping state.
- A job failing every few minutes would otherwise notify every few minutes.
- Apprise can forward to third-party services and phones; job logs can contain secrets.
- Scheduled services do not inherit an interactive PATH, desktop session (D-Bus on Linux), Herdr session, or agent identity.

Scope: example hook script and Noon config snippet under examples/, docs, README links, integration fixtures.

Non-goals: a Noon integration package, changes to the sibling Noon checkout, parsing the JSON summary in Tell, state in Tell, resending notifications or rerunning jobs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An isolated integration fixture runs the example hook with a sanitized service-style environment and a Noon-shaped JSON summary on stdin; a local fixture provider receives the expected destination and job, status, run, and project context, and the JSON summary is not consumed as the message.
- [ ] #2 The example notifies only on status transitions (first failure after success, then recovery) using Noon run history rather than Tell state, keeps hook outcome separate from job outcome, and documents the 30-second hook budget against Tell's timeout.
- [ ] #3 Example messages carry job, status, run ID, and project, and point to noon logs; they never include log excerpts.
- [ ] #4 Docs cover PATH, user config, and desktop session setup for services; explicit Herdr target selection; how callers handle Tell's exit codes (best effort with || true; never rerun a job because a notification failed); task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
