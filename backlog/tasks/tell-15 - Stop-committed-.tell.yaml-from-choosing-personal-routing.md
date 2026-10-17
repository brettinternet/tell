---
id: TELL-15
title: Stop committed .tell.yaml from choosing personal routing
status: Done
assignee:
  - '@brett'
created_date: '2026-10-16 23:19'
updated_date: '2026-10-17 01:49'
labels:
  - reviewed
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
- [x] #1 A repository-authored project file cannot set notify.via, notify.command, apprise.config, herdr.session, send.via, send.to, send.from, or secrets.policy for tell notify or tell send; a regression test covers a committed file in a Git checkout.
- [x] #2 Docs and init templates describe the chosen personal-routing sources; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Remove automatic .tell.yaml loading and init --local; retain explicit personal configuration sources.
2. Update routing diagnostics, documentation, examples, and regression tests for committed repository files.
3. Run focused checks and task ci; independently review the routing trust boundary; commit, merge to main, finalize task state, and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented option (a): removed implicit .tell.yaml loading/project discovery and init --local, retaining user config, explicit --config/TELL_CONFIG, env and flags. Verification: go test ./internal/config ./internal/cli, task smoke, and full task ci passed in the implementation worktree. Committed-Git-checkout binary regression verifies all eight personal fields remain defaults, notify stays stderr/redacts secrets, and neither repository command nor Herdr send executes. Independent reviewer found no production security defects; one migrated test assumed mode 0644 despite umask. Corrected it to compare with normal 0644 creation under the caller umask; focused init test passes under 022 and 077, entire config suite passes, LSP clean, git diff --check clean. Full CI security result: no leaks or reachable vulnerable symbols (govulncheck reports unreachable dependency advisories).

Delivery: implementation commit 5094d2e merged into main after rebasing over an unrelated backlog-only commit. task check:staged passed before implementation commit. Owned worktree and branch fix/tell-15-personal-routing removed via Worktrunk; no remaining blocker or resumable implementation step.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: @pi
created: 2026-10-17 01:49
---
Review: no defects. Implicit .tell.yaml loading and init --local are removed; shared tell.yaml still rejects all personal routing/policy keys; explicit --config/TELL_CONFIG remains trusted. Smoke test covers a committed .tell.yaml in a Git checkout. config/cli tests pass.
---
<!-- COMMENTS:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Removed automatic .tell.yaml loading and init --local to prevent repository-selected personal routing/code execution. Updated docs/templates and migrated personal-config tests. Verified committed-file binary regression, full task ci, and umask 022/077 init regression; independent review found no production defects. Integrated as 5094d2e on main and cleaned up owned worktree.
<!-- SECTION:FINAL_SUMMARY:END -->
