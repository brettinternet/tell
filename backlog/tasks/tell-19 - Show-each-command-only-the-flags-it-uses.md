---
id: TELL-19
title: Show each command only the flags it uses
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:29'
labels: []
dependencies:
  - TELL-14
documentation:
  - docs/configuration.md
modified_files:
  - internal/cli/cli.go
  - README.md
  - docs/configuration.md
  - docs/design.md
  - examples/noon/README.md
  - integration
priority: medium
type: enhancement
ordinal: 12000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: all nine setting flags are global, so `tell notify --help`, `tell send --help`, `tell init --help`, and even `tell config check --help` each list twelve global options, most irrelevant (send help lists --apprise-format; notify help lists --to and --from). Finding the one flag that matters is slow, and nothing signals that --via does nothing for send.

Decision: keep -C/--project, --config, --json, and --timeout global. Put notify routing flags (--via, --apprise-config, --apprise-tag, --apprise-format, --herdr-session) on notify; send routing flags (--send-via, --to, --from, --herdr-session) on send. doctor, config/config show/config check, and `tell mcp` (TELL-14) span both, so they accept every setting flag under the same names; docs already show `tell --via desktop doctor`. Keep flag names unchanged so each name means one thing everywhere. Tell has no published release, so moving flags after the command is acceptable before the compatibility contract starts.

Depends on TELL-14 because it adds `tell mcp` and its flags to the same command tree.

Non-goals: renaming flags or TELL_* variables, changing precedence or provenance labels, accepting command flags before the command name.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `tell --help` lists only global flags plus help/version; `tell notify --help` and `tell send --help` list only flags that affect that command; init help lists no setting flags.
- [x] #2 doctor, config (all forms), and mcp accept every setting flag with unchanged names, values, precedence, and flag provenance.
- [x] #3 Every example in README, docs, examples/noon, and smoke tests places command flags after the command and still runs; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Scope existing setting flags to root, notify, send, or cross-route commands; keep config descendants inheriting settings and preserve input provenance. 2. Move documented/test routing options after commands and verify parsing, precedence, help, and failure output. 3. Run task ci and one independent review, commit and merge locally, finalize task and clean up owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented in .worktrees/tell-19-command-flags. Validation: mise exec -- go test ./internal/cli passed; mise exec -- task ci passed (format, vet, Staticcheck, unit, race, smoke, Gitleaks, govulncheck; no called vulnerabilities). Direct built-binary help inspection confirms root global-only, notify/send scoped, init no local settings (global timeout/config retained per explicit decision), and config check/MCP full settings. Isolated config show with all setting flags overrides invalid routing/timeout environment and reports unchanged flag provenance; isolated doctor accepts all flags and reports expected missing executables with PATH disabled; MCP accepts all flags and exits on stdin EOF. Extended existing override tests across config/show/check including MCP opt-in; migrated routing arguments in docs, examples and tests. Independent reviewer completed one pass: no validated findings. Task no-published-release rationale is stale; explicit command-local flag decision followed. Remaining: commit, merge, finalize and remove owned worktree.

Delivery complete: implementation commit a1c982348e33e5ca34fa7de6c76df350824a9c7c fast-forward merged into main. task check:staged passed before commit. Owned worktree and branch tell-19-command-flags removed using Worktrunk; corresponding Herdr workspace wF1 confirmed absent. No retained checkout/workspace, blockers, or next implementation step. Nothing pushed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Scoped routing flags to the commands that use them; retained all setting flags for doctor/config/MCP and unchanged precedence/provenance. Updated docs, smoke and CLI invocations, and config descendant override regressions. task ci and staged checks passed; independent review found no defects. Merged a1c9823 locally and cleaned up worktree.
<!-- SECTION:FINAL_SUMMARY:END -->
