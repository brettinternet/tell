---
id: TELL-7
title: Keep delivery routing in personal configuration
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 21:29'
updated_date: '2026-10-16 21:57'
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
- [x] #1 Shared tell.yaml rejects notify.via, apprise.config, herdr.session, and hcom.to with a source/line diagnostic that says where to set them; user config, .tell.yaml, --config, TELL_* variables, and flags still accept them. Tests cover rejection and an allowed layer.
- [x] #2 /etc/tell/config.yaml, --system-config, and TELL_SYSTEM_CONFIG are removed; tests and smoke stay isolated from real user settings through HOME and XDG_CONFIG_HOME.
- [x] #3 README, docs/configuration.md, docs/design.md, examples, and the init template show routing in user or private config and shared config limited to shared fields; task ci passes.
- [x] #4 tell init --user creates the user config (honoring XDG_CONFIG_HOME, creating the tell directory as needed) with mode 0600 and commented personal routing examples; it never overwrites an existing file and refuses symlinks, matching project init.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reject routing fields only in the automatic shared layer and remove the system layer/selectors.
2. Reuse exclusive template creation for init --user; keep shared examples routing-free.
3. Verify trust boundaries, allowed sources, init permissions/no-overwrite/symlinks and CLI smoke; update docs and run task ci.
4. Obtain one focused independent security review, commit implementation, merge to main, finalize task evidence and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented in .worktrees/tell-7-personal-routing. Verification passed: mise exec -- go test ./internal/... and mise exec -- task ci (gofmt, vet, Staticcheck, Gitleaks, govulncheck, unit tests, race tests, binary smoke). Regression tests cover all four shared routing rejections with source/line guidance despite overrides; user/private/explicit sources and env/flag precedence; user init HOME/XDG, 0600, no overwrite, target symlink refusal, conflicting targets. Direct built-binary checks passed for both example configs, shared/user init + config check, removed --system-config rejection and ignored obsolete environment variable. Source/test LSP diagnostics clean; smoke diagnostics unknown but compiled/executed by task ci. Focused independent review pending; no acceptance checked yet.

Independent reviewer completed one read-only review: no validated findings. Implementation committed on the worktree, then integrated by cherry-pick as 3e38b73 after concurrent main documentation commits were amended; all unrelated changes preserved. task check:staged passed before commits, and task ci passed again on integrated main. Owned worktree/branch tell-7-personal-routing removed through Worktrunk after receipt and idle-pane verification; post-remove Herdr hook succeeded. No push. No blocker or remaining implementation step.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Personal routing is rejected in automatic shared config with actionable diagnostics. Removed system configuration and added safe init --user. Documentation and regression tests updated. Verified with task ci, direct binary checks, staged checks, and independent review; integrated as 3e38b73.
<!-- SECTION:FINAL_SUMMARY:END -->
