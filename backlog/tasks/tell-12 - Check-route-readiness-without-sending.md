---
id: TELL-12
title: Check route readiness without sending
status: To Do
assignee: []
created_date: '2026-10-16 21:41'
labels: []
dependencies:
  - TELL-2
  - TELL-3
  - TELL-8
  - TELL-11
documentation:
  - docs/design.md
type: feature
ordinal: 3500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: notification setup fails silently at the worst time: a scheduled job fails at 3 a.m. and its alert also fails because a provider is missing or misconfigured. A read-only readiness check lets you verify setup once, and lets Noon hook docs point at one command.

Context: no-argument and inspection commands must never send messages or start services (AGENTS.md). Apprise supports --dry-run, which lists matching services without sending; its output can contain credential-bearing URLs, so report only a count. herdr session list is read-only.

Scope: a doctor command (or equivalent read-only subcommand) in internal/cli, per-route readiness checks in internal/delivery, tests with fixture executables, docs.

Non-goals: sending test notifications, installing or starting providers, repairing configuration, network reachability probes beyond what a provider's own dry run does.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 tell doctor reports, for the selected route and every configured route, whether it is ready and why not (missing executable, unsupported platform, Herdr session not running, Apprise matching zero services, command not executable), and exits 0 only when the selected route is ready.
- [ ] #2 Fixture-based tests prove doctor never sends, never prints Apprise URLs or other provider configuration, and emits one schema_version 1 object with --json.
- [ ] #3 README and setup docs recommend tell doctor after configuration; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
