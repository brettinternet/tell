---
id: TELL-5
title: Document and verify repo automation with Noon hooks
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 20:47'
labels: []
dependencies:
  - TELL-2
  - TELL-3
  - TELL-4
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
- Scheduled services do not inherit an interactive PATH, Herdr session, or agent identity.

Scope: example hook script and Noon config snippet under examples/, docs, README links, integration fixtures.

Non-goals: a Noon integration package, changes to the sibling Noon checkout, parsing the JSON summary in Tell, resending notifications or rerunning jobs.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An isolated integration fixture runs the example hook with a sanitized service-style environment and a Noon-shaped JSON summary on stdin; a local fixture provider receives the expected destination and job/status/run context, and the JSON summary is not consumed as the message.
- [ ] #2 Examples default to on_failure, keep hook outcome separate from job outcome, and document the 30-second hook budget against Tell's timeout.
- [ ] #3 Docs cover PATH and user-config setup for services, explicit Herdr/hcom target selection, and why a failed notification does not justify rerunning the original job; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
