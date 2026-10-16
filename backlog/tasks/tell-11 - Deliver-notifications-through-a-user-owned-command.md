---
id: TELL-11
title: Deliver notifications through a user-owned command
status: To Do
assignee: []
created_date: '2026-10-16 21:41'
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
- [ ] #1 notify --via command runs the configured argv without a shell; a local fixture verifies exact argv, the body on stdin, and title, type, and project in the environment, and that Tell's own stdin is never inherited.
- [ ] #2 Child exit 0, 3, and other non-zero map to accepted, not_sent, and failed; a missing executable is not_sent; timeout and cancellation are uncertain; shared tell.yaml rejects the command setting.
- [ ] #3 Docs include a worked personal example and the exact contract; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
