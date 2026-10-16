---
id: TELL-5
title: Document and verify repo automation with Noon hooks
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 20:29'
labels: []
dependencies:
  - TELL-2
  - TELL-3
  - TELL-4
ordinal: 5000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The primary usage is recurring repo work supervised by Noon. Provide a small repo-owned hook script and example Noon configuration that translate NOON_JOB, NOON_STATUS, NOON_RUN_ID and NOON_PROJECT into Tell arguments. Scope: examples, docs, README links, and focused integration fixtures. Keep Tell independent of Noons JSON schema and run store. No scheduler integration package or changes to the sibling Noon checkout.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 An isolated executable integration fixture runs the example hook with a sanitized service-style environment and verifies its destination and message context without calling any live provider.
- [ ] #2 Examples use failure notifications by default, preserve job-vs-hook outcome separation, and document the 30-second Noon hook budget versus the Tell timeout.
- [ ] #3 Documentation covers PATH/user config setup, explicit Herdr/hcom target selection, and why a failed notification does not justify repeating the original job; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
