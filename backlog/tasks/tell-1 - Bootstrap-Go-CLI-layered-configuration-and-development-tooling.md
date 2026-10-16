---
id: TELL-1
title: 'Bootstrap Go CLI, layered configuration, and development tooling'
status: In Progress
assignee: []
created_date: '2026-10-16 20:19'
updated_date: '2026-10-16 20:19'
labels: []
dependencies: []
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Tell needs a safe reusable foundation for notifications and agent messages invoked by people, scripts, and Noon. Reuse the architecture and tooling conventions from sibling hum, bump (Noon), and project. Scope: cmd/tell, internal/cli, internal/config, tests, docs, examples, mise/Task/Lefthook/CI/Worktrunk tooling, and backlog configuration. Implement inspection, strict layered settings, and non-destructive init; do not add delivery adapters, queues, services, or provider SDKs. System/user settings, tell.yaml, private .tell.yaml, environment, and flags must resolve predictably with source-relative paths and provenance.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 mise exec -- task ci passes: source/static/race checks, security scans, and isolated binary smoke.
- [ ] #2 mise exec -- task test verifies configuration precedence, explicit-config isolation, strict YAML rejection, Git-boundary discovery, source-relative paths, and no-overwrite init through public interfaces.
- [ ] #3 Built tell supports help/version, safe no-argument config inspection, config check, --json, init and init --local; unsupported delivery commands fail rather than pretend to send.
- [ ] #4 README and docs distinguish implemented bootstrap from future adapters; Backlog.md tracks bounded dependent delivery work; intended files are committed with Conventional Commits and no push.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the sibling Go-only mise/Task/Lefthook/CI/Worktrunk skeleton and commit the tooling baseline.
2. Adapt Noons strict configuration and urfave/cli patterns for Tell settings, source provenance, safe inspection, and no-overwrite init.
3. Add critical parsing/precedence/init regressions after implementation and an isolated binary smoke check.
4. Document implemented behavior, seed dependent adapter tasks through Backlog.md, run the complete gate, obtain independent configuration-boundary verification, and commit the completed bootstrap.
<!-- SECTION:PLAN:END -->
