---
id: TELL-9
title: Normalize outbound message text before delivery
status: To Do
assignee: []
created_date: '2026-10-16 21:41'
labels: []
dependencies:
  - TELL-2
documentation:
  - docs/design.md
type: enhancement
ordinal: 2100
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: titles and bodies often come from job output or logs. Terminal-rendered routes (stderr, Herdr) interpret escape sequences, so untrusted text can rewrite the screen or spoof content; bidi controls can disguise text in banners; huge bodies are truncated or rejected inconsistently by providers. One preparation step shared by every route keeps behavior predictable.

Context: config values already reject NUL, ESC, CR, and LF (internal/config). Unharness bounds message size and treats content as inert data (../unharness/docs/threat-model.md).

Scope: one shared preparation step in internal/delivery or a small sibling package, CLI wiring, tests, docs.

Non-goals: markdown or HTML sanitization (format is chosen by personal config), secret scanning (separate task), translation or reformatting of content.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Before any route runs, NUL in a title or body fails with exit 1; other C0 and C1 controls, terminal escape sequences, and Unicode bidi override and isolate controls are removed while newline and tab are preserved. Tests cover an ANSI escape sequence and a bidi override.
- [ ] #2 Titles and bodies are capped at documented byte limits on UTF-8 boundaries with a visible truncation marker; tests cover a multibyte boundary.
- [ ] #3 Docs state the limits and the removal rules; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
