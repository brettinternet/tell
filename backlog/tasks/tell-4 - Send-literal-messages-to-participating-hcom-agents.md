---
id: TELL-4
title: Send literal messages to participating hcom agents
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 20:29'
labels: []
dependencies:
  - TELL-2
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Humans and scheduled scripts need to message an existing hcom participant from another shell, without coupling that sender to terminal-agent lifecycle management. Scope: send command, hcom delivery adapter, exercised config changes, local executable fixtures, tests, and docs. Explicit --to overrides configured hcom.to. Keep notification attention separate from agent messaging. No agent spawning, queue, reply waiting, workflow completion claims, or automatic retries.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test exercises tell send with literal and stdin messages; a local hcom fixture observes the exact recipient and body argv, including whitespace, quotes, and leading dashes.
- [ ] #2 Missing recipients, absent executables, provider failures, timeout and cancellation are distinguishable nonzero outcomes; an uncertain send is never automatically retried or reported as completed agent work.
- [ ] #3 task ci passes and help/docs explain participation requirements, explicit target selection, and that dispatch acknowledgment is not execution completion.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
