---
id: TELL-3
title: Deliver contextual notifications to an explicit Herdr session
status: To Do
assignee: []
created_date: '2026-10-16 20:28'
updated_date: '2026-10-16 21:30'
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
- Reuse TELL-2's notify contract, subprocess helper, JSON result, and redaction.

Scope: the Herdr route in internal/delivery, CLI/config changes only if exercised, local fixture executables, tests, docs, and examples.

Non-goals: pane, tab, or workspace control; agent prompting; starting or creating Herdr sessions or servers; guessing a session; falling back to desktop or another route.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test exercises notify --via herdr through a local fixture executable and verifies that the title, body, and configured herdr.session reach Herdr as separate literal argv elements.
- [ ] #2 A missing herdr.session or herdr executable returns not_sent before anything runs; inherited HERDR_* variables that point at another session never change the target; an unreachable session fails visibly per docs/design.md outcomes without creating or starting a session or choosing another destination.
- [ ] #3 task ci passes; Herdr delivery keeps TELL-2's timeout and redaction contract, and docs state that a posted notification is not human acknowledgment.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
