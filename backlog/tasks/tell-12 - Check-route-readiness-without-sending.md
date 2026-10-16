---
id: TELL-12
title: Check route readiness without sending
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 21:41'
updated_date: '2026-10-16 23:11'
labels:
  - reviewed
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
- [x] #1 tell doctor reports, for the selected route and every configured route, whether it is ready and why not (missing executable, unsupported platform, Herdr session not running, Apprise matching zero services, command not executable), and exits 0 only when the selected route is ready.
- [x] #2 Fixture-based tests prove doctor never sends, never prints Apprise URLs or other provider configuration, and emits one schema_version 1 object with --json.
- [x] #3 README and setup docs recommend tell doctor after configuration; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add bounded read-only readiness checks for selected and explicitly configured routes, using Apprise --dry-run and Herdr session list --json; never expose provider output or execute personal commands. 2. Add doctor human/JSON output with selected-route exit status and document setup semantics. 3. Exercise fixture-based safety, parsing, and failure regressions; run task ci and one independent review. 4. Commit implementation, merge main, record acceptance evidence, and clean up owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented doctor in tell-12-doctor worktree. Verification: go test ./internal/cli ./internal/delivery and task ci pass (format, vet, Staticcheck, Gitleaks, govulncheck, unit, race, build, smoke). Fixture tests cover selected-vs-configured status, exact dry-run/list argv, EOF stdin, sanitized output, single schema_version 1 JSON object, missing/non-executable commands, zero services, stopped/missing sessions, malformed/oversized output, timeouts, unsupported desktop platforms, and no personal/desktop execution. Oversize regression caught and fixed bytes.Buffer ReadFrom bypass of the capture limit. Clean LSP diagnostics. Built CLI in empty environment with explicit empty config reports stderr ready in human and JSON modes. README/configuration/development setup docs recommend doctor and explain its limits. Independent safety review pending; next: resolve concrete findings, commit, fast-forward main, finalize task and clean owned worktree.

Independent reviewer completed the single safety review with no validated remaining findings; confirmed named-buffer fix. Implementation committed as 4c1300e (feat: check route readiness without sending) after task check:staged and fast-forward merged into main. No push. Owned worktree/branch tell-12-doctor removed through Worktrunk; exact Herdr workspace wE3 verified absent afterward. No retained checkout or workspace, remaining blocker, or resumable implementation step.

Review: no defects found. Verified read-only probes, sanitized output, selected-route exit semantics, and Herdr session list --json shape against the real Herdr CLI.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added read-only tell doctor with sanitized human/JSON route readiness, selected-route exit status, bounded Apprise dry-run and Herdr session checks, executable-only command/desktop checks, and setup documentation. Verified fixture regressions, direct built CLI, clean diagnostics, task ci, and independent safety review. Committed 4c1300e and merged main; worktree/workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->
