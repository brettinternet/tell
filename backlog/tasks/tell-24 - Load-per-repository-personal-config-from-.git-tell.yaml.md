---
id: TELL-24
title: Load per-repository personal config from .git/tell.yaml
status: Done
assignee:
  - '@pi'
created_date: '2026-10-17 02:02'
updated_date: '2026-10-17 02:08'
labels: []
dependencies:
  - TELL-15
modified_files:
  - internal/config/config.go
  - internal/config/init.go
  - internal/cli/cli.go
  - internal/config/config_test.go
  - internal/cli/cli_test.go
  - integration/smoke_test.go
  - README.md
  - docs/configuration.md
  - docs/design.md
  - examples/tell.yaml
type: enhancement
ordinal: 17000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: TELL-15 removed the in-tree .tell.yaml layer because a PR can commit that filename; Tell trusted it by name alone and ran its notify.command. That left no per-repository personal settings: user config is global, and --config/TELL_CONFIG replaces every automatic file. Per-repo personal routing (for example a Herdr session or agent recipient per repository) is still wanted. A file inside the repository Git directory cannot be committed or checked out (Git refuses paths inside .git), and linked worktrees share the common Git directory, so one file covers the main checkout and every .worktrees/* checkout. This mirrors Git own per-repository .git/config.

Non-goals: trust or approval stores; walking above the project to find a Git directory; per-worktree layers; reading GIT_DIR/GIT_COMMON_DIR or executing git; changing env/flag precedence.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Tell loads <git common dir>/tell.yaml as personal config after shared tell.yaml and before TELL_* environment, from the main checkout and from linked worktrees (resolved from the project .git directory or .git file and commondir without running git). It accepts every personal field; --config replaces it; a committed .tell.yaml remains ignored. Tests cover precedence and a real linked worktree.
- [x] #2 tell init --local creates that file with mode 0600 and personal template, never overwrites, refuses non-regular existing paths, fails outside a Git checkout with guidance, and cannot combine with --user.
- [x] #3 README, docs/configuration.md, docs/design.md, init help, and shared-file diagnostics name .git/tell.yaml as a personal source; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Resolve the common Git directory from <project>/.git (directory, or gitdir file plus optional commondir) with bounded file reads; load tell.yaml there as a personal layer between shared tell.yaml and overrides.
2. Add InitLocal and tell init --local (0600, personal template variant, no overwrite, error without Git, exclusive with --user).
3. Update shared-file diagnostics, init help/next steps, README, configuration and design docs, example comment.
4. Tests: config precedence/malformed .git; CLI init flags; smoke with a real git worktree (init --local from worktree, personal command runs, committed .tell.yaml still ignored).
5. Focused tests, task ci, task check:staged, commit; finalize task.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented in current checkout as 23343fa. User direction: ignore .tell.yaml entirely (no compatibility warnings); removed compatibility-only doc/test mentions, kept the TELL-15 committed-.tell.yaml smoke regression. Pointer files (.git, commondir) must be regular files and at most 4 KiB so a FIFO cannot hang Tell. Verification: go test ./internal/... passed; task smoke passed, including a real git worktree where init --local from the worktree created the main checkout .git/tell.yaml with mode 0600 and notify then ran the personal command; TestRepositoryPersonalConfig covers main/worktree precedence (user, then tell.yaml, then .git/tell.yaml, then flags), --config isolation, and malformed .git; CLI test covers init --local outside Git (exit 1, points to --user) and --local/--user conflict (exit 2). Direct built-binary check in a temp repo: init --local/-l from worktree created then kept the file; tell config in worktree and main showed herdr.session from .git/tell.yaml; outside Git exit 1. task ci passed (gofmt, vet, staticcheck, gitleaks, govulncheck no reachable symbols, tests, race, smoke); task check:staged passed; LSP clean.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added a repository-personal config layer in the common Git directory (normally .git/tell.yaml), loaded after shared tell.yaml and before TELL_* env, resolved from .git directories or worktree .git files plus commondir without running git; accepts all personal fields; --config replaces it. Added tell init --local/-l (0600, no overwrite, fails outside Git). Updated diagnostics, templates, help, README, configuration and design docs. Verified with unit tests, real-worktree smoke test, direct binary checks, and task ci.
<!-- SECTION:FINAL_SUMMARY:END -->
