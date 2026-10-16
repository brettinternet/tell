---
id: TELL-15
title: Stop committed .tell.yaml from choosing personal routing
status: To Do
assignee: []
created_date: '2026-10-16 23:19'
labels: []
dependencies:
  - TELL-7
references:
  - docs/configuration.md
  - internal/config/config.go
type: enhancement
ordinal: 8000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: TELL-7 keeps repositories from choosing where messages go by rejecting routing in shared tell.yaml, but a PR can commit .tell.yaml instead. Tell loads it as private config, so it can set notify.command (arbitrary code execution on tell notify), apprise.config (send bodies to repo-chosen destinations), or send.via/send.to (inject requests into an agent). Reproduced in review against a built binary: a committed .tell.yaml with notify.command [/usr/bin/touch, FILE] created FILE on tell notify, exit 0.

Options from review: (a) remove the .tell.yaml layer and tell init --local, leaving personal routing to user config, --config/TELL_CONFIG, env, and flags (simplest; unreleased so breaking is acceptable); (b) refuse .tell.yaml when Git tracks it (config load would run git; non-Git checkouts and archives stay exposed).

Scope: internal/config, internal/cli, tests, smoke, README, docs/configuration.md, docs/design.md, examples, init templates.
Non-goals: changing env/flag precedence; new settings.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A repository-authored project file cannot set notify.via, notify.command, apprise.config, herdr.session, send.via, send.to, send.from, or secrets.policy for tell notify or tell send; a regression test covers a committed file in a Git checkout.
- [ ] #2 Docs and init templates describe the chosen personal-routing sources; task ci passes.
<!-- AC:END -->
