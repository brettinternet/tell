---
id: TELL-7
title: Keep delivery routing in personal configuration
status: To Do
assignee: []
created_date: '2026-10-16 21:29'
updated_date: '2026-10-16 21:43'
labels: []
dependencies:
  - TELL-1
documentation:
  - docs/design.md
  - docs/configuration.md
  - ../bump/docs/configuration.md
type: enhancement
ordinal: 1500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: delivery destinations are personal, but shared tell.yaml currently overrides user config. A repository therefore decides where your notifications and agent messages go, and can point apprise.config at a repository-controlled destination list. Agents working in PR checkouts would inherit PR-authored routing. Noon applies the same rule: top-level hooks come only from user config or --config, never project noon.yaml. Separately, the /etc/tell system layer has no user; --system-config exists mainly to isolate tests from it.

Land this before any route sends messages, while nothing depends on the current precedence. See the routing ownership paragraph in docs/design.md.

Scope: internal/config, internal/cli, tests, integration/smoke_test.go, README, docs/configuration.md, docs/design.md, examples, init template.

Non-goals: new settings or commands; changing env/flag precedence; changing other fields' layering.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Shared tell.yaml rejects notify.via, apprise.config, herdr.session, and hcom.to with a source/line diagnostic that says where to set them; user config, .tell.yaml, --config, TELL_* variables, and flags still accept them. Tests cover rejection and an allowed layer.
- [ ] #2 /etc/tell/config.yaml, --system-config, and TELL_SYSTEM_CONFIG are removed; tests and smoke stay isolated from real user settings through HOME and XDG_CONFIG_HOME.
- [ ] #3 README, docs/configuration.md, docs/design.md, examples, and the init template show routing in user or private config and shared config limited to shared fields; task ci passes.
- [ ] #4 tell init --user creates the user config (honoring XDG_CONFIG_HOME, creating the tell directory as needed) with mode 0600 and commented personal routing examples; it never overwrites an existing file and refuses symlinks, matching project init.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
