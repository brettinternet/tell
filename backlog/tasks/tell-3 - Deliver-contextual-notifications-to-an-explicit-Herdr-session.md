---
id: TELL-3
title: Deliver contextual notifications to an explicit Herdr session
status: To Do
assignee: []
created_date: '2026-10-16 20:28'
updated_date: '2026-10-16 20:29'
labels: []
dependencies:
  - TELL-2
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Repo jobs need contextual terminal feedback without relying on the shell that originally launched them. Scope: the concrete Herdr delivery adapter, CLI/config additions only if needed, local executable fixtures, tests, and docs. Use the installed Herdr notification CLI and configured herdr.session; share the existing bounded subprocess contract. Never guess another session, spawn Herdr, or fall back silently to desktop delivery. No pane control or agent prompting.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test exercises notify --via herdr through a local executable fixture and verifies title/body and configured session are passed as separate literal arguments.
- [ ] #2 Scheduled-environment tests run without inherited HERDR_* variables; explicit session routing works and missing/unreachable targets produce visible nonzero failures without choosing another destination.
- [ ] #3 task ci passes; provider diagnostics preserve the existing redaction and timeout contract, and documentation states that a posted banner is not human acknowledgment.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
