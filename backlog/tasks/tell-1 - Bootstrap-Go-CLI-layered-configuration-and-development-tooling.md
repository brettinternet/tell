---
id: TELL-1
title: 'Bootstrap Go CLI, layered configuration, and development tooling'
status: Done
assignee: []
created_date: '2026-10-16 20:19'
updated_date: '2026-10-16 23:52'
labels:
  - reviewed
dependencies: []
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Tell needs a safe reusable foundation for notifications and agent messages invoked by people, scripts, and Noon. Reuse the architecture and tooling conventions from sibling hum, bump (Noon), and project. Scope: cmd/tell, internal/cli, internal/config, tests, docs, examples, mise/Task/Lefthook/CI/Worktrunk tooling, and backlog configuration. Implement inspection, strict layered settings, and non-destructive init; do not add delivery adapters, queues, services, or provider SDKs. System/user settings, tell.yaml, private .tell.yaml, environment, and flags must resolve predictably with source-relative paths and provenance.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 mise exec -- task ci passes: source/static/race checks, security scans, and isolated binary smoke.
- [x] #2 mise exec -- task test verifies configuration precedence, explicit-config isolation, strict YAML rejection, Git-boundary discovery, source-relative paths, and no-overwrite init through public interfaces.
- [x] #3 Built tell supports help/version, safe no-argument config inspection, config check, --json, init and init --local; unsupported delivery commands fail rather than pretend to send.
- [x] #4 README and docs distinguish implemented bootstrap from future adapters; Backlog.md tracks bounded dependent delivery work; intended files are committed with Conventional Commits and no push.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse the sibling Go-only mise/Task/Lefthook/CI/Worktrunk skeleton and commit the tooling baseline.
2. Adapt Noons strict configuration and urfave/cli patterns for Tell settings, source provenance, safe inspection, and no-overwrite init.
3. Add critical parsing/precedence/init regressions after implementation and an isolated binary smoke check.
4. Document implemented behavior, seed dependent adapter tasks through Backlog.md, run the complete gate, obtain independent configuration-boundary verification, and commit the completed bootstrap.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
AC#1: mise exec -- task ci passed: Gitleaks, govulncheck (no vulnerabilities), gofmt/vet/Staticcheck, git-cliff, unit tests, race tests, and isolated binary smoke. mise exec -- task init also completed successfully.
AC#2: mise exec -- task test passed public-interface regression tests for all config layers, field/path provenance, explicit-file isolation, malformed/ambiguous YAML, worktree boundaries, dangling symlinks, flag/env precedence, JSON failures, and no-overwrite initialization. Tests were added after implementation; none were weakened.
AC#3: mise exec -- task smoke passed built-binary help/version, no-argument JSON inspection, config check, shared/private init, invalid-config exits, and unsupported delivery-command rejection with sanitized HOME/config/environment.
AC#4: README/docs/examples clearly distinguish implemented config from future adapters; Backlog CLI created TELL-2 through TELL-6 with dependencies and acceptance. Commits d726d5d (tooling) and ea23bb4 (configuration CLI/docs/tests) contain the implementation; the final backlog-state commit records this completion. No push or PR was performed.
Independent verifier e250b7e2 reran tests, full CI, and smoke and found no implementation defect; its only outstanding criterion was the then-uncommitted implementation, resolved by ea23bb4. Its observed backlog changes were this parent sessions authorized task creation/updates. LSP diagnostics were clean. No worktrees or services were created.

Review: one defect fixed in 78a3c65. Ambient environment blocked init: any exported TELL_* setting (e.g. TELL_HERDR_SESSION) or TELL_CONFIG made tell init/--local/--user exit 2 with 'init does not apply config or setting overrides', and TELL_PROJECT made init --user claim --project was passed. TELL_PROJECT/TELL_CONFIG are now resolved in input() like the setting env vars, so init rejects only typed flags; regression added to TestCLIUserInit (fails before the fix). Otherwise traced Load precedence (user < tell.yaml < .tell.yaml < env < flag, --config replacing files only), shared-file personal-key rejection, strict YAML (aliases, merge keys, dotted/duplicate keys, multi-doc, 1 MiB cap, dangling symlinks), Git-boundary discovery, source-relative apprise.config, and O_EXCL no-overwrite init refusing symlinks. Stale template/next-step wording is already TELL-23; committed .tell.yaml routing is TELL-15. task ci passed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Bootstrapped a Go/urfave CLI with system/user/shared/private YAML plus env/flag precedence, source-aware inspection, strict validation, and safe init. Reused sibling tooling and created five dependent delivery/integration/packaging tasks. Full local CI, independent verification, and staged secret checks passed. Provider delivery remains intentionally unimplemented.
<!-- SECTION:FINAL_SUMMARY:END -->
