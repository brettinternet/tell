---
id: TELL-25
title: Improve human CLI readability with restrained terminal styling
status: Done
assignee: []
created_date: '2026-10-17 05:55'
updated_date: '2026-10-17 06:08'
labels: []
dependencies: []
ordinal: 18000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Human output currently has little visual hierarchy. Add bold headings/labels and semantic status accents to inspection, help, doctor, receipts, and errors while preserving readable monochrome output and automation contracts. Scope: internal/cli, focused integration tests if needed, go.mod/go.sum for terminal detection, README and CLI documentation, generated changelog, and this task. Non-goals: changing delivery payloads, JSON/MCP schemas, providers, routing, boxes/banners/emoji, or a theme system. No dependencies. User authorized implementation, commits, push, and a new stable release. Guidance: clig.dev output, no-color.org, W3C use-of-color.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Terminal human output uses bold headings/labels and green accepted/ready, yellow warning/uncertain, red errors/selected unready; optional unready routes remain neutral and doctor remediation is indented.
- [x] #2 Automatic styling checks stdout/stderr independently; nonempty NO_COLOR, TERM=dumb, and an explicit plain-output flag suppress styling; important text stays in the default foreground.
- [x] #3 JSON, MCP, shell completion, notification bodies and provider payloads receive no added ANSI; accepted remains dispatched rather than completed; monochrome output retains text labels and readable alignment.
- [x] #4 Regression checks cover output/protocol isolation and plain controls; terminal output is exercised; docs describe the behavior; task ci and release packaging/actionlint pass.
- [x] #5 Changes are committed and pushed; a new stable release is published through the existing workflow and its installation matrix passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse go-isatty for a small CLI-only style helper; add --no-color (disables bold too), independent stream checks, and NO_COLOR/TERM guards. Keep provider writers untouched. 2. Apply labels/headings and semantic accents at human rendering sites; format doctor remediation on continuation lines; style help only after framework layout to preserve alignment. 3. Add focused regressions for protocol/payload isolation and flag parsing, exercise real pseudo-terminal output with plain/JSON controls, and document behavior. 4. Run task ci, independent review focused on ANSI leakage and flag handling, staged checks; commit and push main. Package v0.3.0, validate workflow, tag/push for publication, verify release and four installation runners, then record completion.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented CLI-only styling with existing go-isatty, no delivery-writer wrappers, post-layout help/table accents, and indented doctor remediation. Added creack/pty v1.1.24 only for real terminal regression checks; macOS PTY capture drains through a marker before close to avoid discarded unread output. Verification: go test ./internal/cli -run TestTerminal -count=1 -v passed; task ci passed (Gitleaks, govulncheck, gofmt, vet, Staticcheck, unit/race tests, binary smoke). LSP diagnostics clean on changed source/tests. Terminal tests verify independent streams, NO_COLOR=0, TERM=dumb, --no-color before/after commands and on parse failures, layout equality after removing ANSI, JSON/MCP/completions/body isolation, selected/optional readiness, accepted/uncertain receipts, and unchanged provider payloads. Independent review pending; release packaging/publication pending.

Independent reviewer found two opt-out defects (help after an unknown flag and doctor after a root --). Reproduced on a real PTY, fixed invocation-level help policy and command-aware terminator scanning, and added regressions. Also preserved an earlier valid output switch when a later boolean is invalid. Focused PTY suite and full task ci passed again after fixes; actionlint .github/workflows/release.yaml passed. Direct built-binary PTY execution confirmed bold help, aligned config columns, and neutral optional doctor failures. Review complete; no remaining validated findings.

Implementation committed and pushed to origin/main as af215cf03c55d6b7a0c8e47f06135ef551031c14. task check:staged passed before commit. Clean-checkout VERSION=v0.3.0 task cli:package passed: four archives/checksums verified and host binary reported 0.3.0 (af215cf). actionlint passed. Waiting for exact-commit main CI run 39008486651 before tagging v0.3.0; then verify release publication and four mise-install jobs. User explicitly authorized push/release.

Exact-commit main CI run 39008486651 passed on Linux and macOS. Created annotated v0.3.0 at af215cf03c55d6b7a0c8e47f06135ef551031c14 and pushed the tag to trigger the existing release workflow. Publication and four installation checks are pending; no release assets have been manually uploaded or replaced.

Published stable release https://github.com/brettinternet/tell/releases/tag/v0.3.0 at 2026-10-17T06:07:39Z from af215cf. gh release view confirmed four platform archives plus checksums.txt, not draft/prerelease. Release workflow https://github.com/brettinternet/tell/actions/runs/39008645164 succeeded: publication and mise-install on ubuntu-latest, ubuntu-24.04-arm, macos-15-intel, and macos-14 all passed. All acceptance and delivery obligations verified; no blockers or retained worktrees.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented bold CLI hierarchy, semantic status accents, indented doctor guidance, and --no-color/environment/TTY safeguards without styling payloads or machine protocols. Verified real PTYs, full local CI, independent review fixes, Linux/macOS CI, packaging/actionlint, and four live installation runners. Committed/pushed as af215cf and released v0.3.0.
<!-- SECTION:FINAL_SUMMARY:END -->
