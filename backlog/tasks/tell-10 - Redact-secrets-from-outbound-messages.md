---
id: TELL-10
title: Redact secrets from outbound messages
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 21:41'
updated_date: '2026-10-16 23:11'
labels:
  - reviewed
dependencies:
  - TELL-9
references:
  - 'https://github.com/betterleaks/betterleaks'
documentation:
  - docs/design.md
  - ../unharness/docs/threat-model.md
type: enhancement
ordinal: 2200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: Tell is the last point before a message reaches services that persist it: Apprise forwards to chat, email, and phones, and agent transports keep transcripts. Scripts will eventually put a log line with a token into a message despite guidance. Unharness scans every published message with an embedded Gitleaks ruleset for the same reason (../unharness/scan/scan.go, ../unharness/docs/threat-model.md: plaintext keys in stored agent threads).

Context:
- Betterleaks is maintained by the Gitleaks maintainers, including its original author. Its Go library exposes ScanString and reusable scanners; v2 on main is pre-stable while v1 is maintained. Unharness embeds the Gitleaks v8 library (detect.Detector).
- Embed the scanner. An optional external scanner would silently disappear when not installed, which defeats a safety control.
- Default policy redacts so failure alerts still arrive; reject is a personal choice.
- Repository dev tooling keeps Gitleaks for parity with sibling projects; that is separate from this runtime check.

Scope: the shared message preparation step, personal config for the policy, tests with synthetic tokens, docs.

Non-goals: custom rules or allowlists, live secret validation or any network call, scanning files or logs, sealed-secret delivery.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Before any route runs, the title and body are scanned with an embedded default ruleset (Betterleaks or Gitleaks library; notes record the choice and why). By default each finding is replaced with a fixed marker and delivery continues; the JSON result reports the redaction count, never the secret.
- [x] #2 A reject policy, settable only in personal config, fails with exit 1 and reason secret_detected before any provider runs; shared tell.yaml cannot weaken the policy.
- [x] #3 Tests use synthetic tokens matching default rules and cover redact and reject for title and body. Added per-invocation latency and binary size are measured and recorded in notes; task ci, including govulncheck, passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Embed Gitleaks v8.30.1 default rules following sibling Unharness; scan normalized text before truncation and redact before dispatch. 2. Add file-only personal secrets.policy (redact default, reject), safe result counts, and synthetic-token regression tests. 3. Update docs, measure cold invocation latency and binary size, run task ci and one security-focused review. 4. Commit implementation, merge to main, record completion, remove owned worktree, release claim.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented embedded Gitleaks v8.30.1 default rules (same stable library as Unharness, avoiding pre-stable Betterleaks v2). No external scanner, custom configuration, filesystem scan, or network validation. Inline gitleaks:allow is ignored. Normalize before scanning, redact before truncation; merged occurrence counts appear in safe JSON. Personal file-only secrets.policy supports redact/reject; shared policy and non-file overrides fail. Focused go test ./internal/... and task ci PASS (vet, Staticcheck, tests, race, binary smoke, Gitleaks, govulncheck). Upgraded transitive xz to v0.5.15 to resolve GO-2025-3922; govulncheck reports zero reachable vulnerabilities (unreachable dependency advisories remain). LSP diagnostics clean for changed Go files. Synthetic-token tests exercise title/body redact/reject, no provider execution on reject, argv and stderr redaction, inline allow bypass, ANSI-split tokens, truncation boundary, and personal/shared policy trust. Performance on this macOS host: same unstripped go build, baseline 44920f9 vs changed tree; 30 separate CLI processes after one warmup, isolated explicit config and clean environment, notify Build finished to stderr discarded: baseline median 3.504 ms / mean 3.593 ms; changed median 16.722 ms / mean 16.749 ms (+13.218 ms median). Binary 7,131,714 -> 20,016,290 bytes (+12,884,576 bytes). One independent security review pending; next: resolve concrete findings, commit, merge, finalize, clean owned worktree.

Independent reviewer completed one security-focused pass: one P2, repeated findings caused quadratic replacement spans. Fixed by deduplicating secret strings before locating occurrences. Direct isolated binary probes verified exact counts 1000/2000/10000 and max RSS 44.0/47.9/66.2 MB after fix (before: 88.1/208.0 MB for 1000/2000). task ci rerun after correction PASS; LSP clean; task check:staged PASS. Implementation committed abe7ee2 and fast-forward merged into main. Owned worktree .worktrees/feat-tell-10 and branch removed through Worktrunk; associated Herdr workspace wDX closed. Unrelated primary backlog edits preserved. No push. No remaining blocker or resumable implementation step.

Review: found cross-field leak; a secret detected by a context rule in one field (title api_key=VALUE) was not redacted where VALUE repeated in the other field. Fixed in d99eada by collecting secrets across title and body before redacting both, and failing closed (secret_scan_failed) if a finding cannot be located. Regression test TestNotifySecrets/cross-field fails before, passes after; 10000 repeated tokens still 0.3s/67MB; task ci PASS.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Embedded offline Gitleaks redacts title/body secrets by default and reports safe JSON counts; personal file-only reject policy prevents dispatch. Verified synthetic-token security boundaries, latency/size, repeat-token memory scaling, and full task ci. Security review finding fixed. Committed abe7ee2, merged to main, owned worktree/branch/workspace cleaned.
<!-- SECTION:FINAL_SUMMARY:END -->
