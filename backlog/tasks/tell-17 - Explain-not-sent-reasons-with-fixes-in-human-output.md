---
id: TELL-17
title: Explain not-sent reasons with fixes in human output
status: Done
assignee: []
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:49'
labels:
  - reviewed
dependencies: []
documentation:
  - docs/design.md
modified_files:
  - internal/cli/send.go
  - internal/cli/doctor.go
  - internal/cli/notify.go
  - internal/delivery/send.go
  - internal/delivery/doctor.go
  - README.md
  - docs/design.md
priority: high
type: enhancement
ordinal: 10000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: when delivery or readiness fails, human output shows only machine reason codes, so a person must look up what `missing_route` means and which setting fixes it. Found in a CLI UX audit:

- `tell send hi` with no send config prints `tell send: route="" recipient="" intent="inform" outcome=not_sent reason=missing_route`. The Result already carries a human message, but renderSend never prints it; the same receipt hides `send requires one message argument` for bare `tell send`.
- `--send-via hcom` without a recipient gives `reason=missing_target`; `--send-via herdr` gives `reason=missing_session`; neither says to set send.to or herdr.session.
- `tell doctor` prints lines such as `apprise: missing_executable` with no plain explanation or fix.

Scope: human rendering in internal/cli (send receipt, notify failures, doctor lines) and, where needed, the human message text attached to delivery results in internal/delivery; README/design examples that show these lines.

Non-goals: changing outcome or reason codes, exit codes, or JSON fields; removing the body-free send receipt line scripts may grep; installing providers or probing networks; printing bodies, provider output, or provider configuration.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Every non-accepted human send result prints the reason in words after the receipt; bare `tell send` explains that a message argument or - is required.
- [x] #2 missing_route, missing_target, and missing_session explanations name the setting to configure (send.via, send.to, herdr.session) with its flag and TELL_* environment variable.
- [x] #3 Human doctor output marks each route ready or not ready and explains each not-ready reason in words (for example that apprise was not found on PATH); --json reason codes are unchanged.
- [x] #4 README and docs/design.md examples match the new human output; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Inspect delivery reasons and human renderers; add safe actionable explanations without changing JSON or receipts.
2. Extend focused regression checks and update documented output.
3. Run task ci, review the scoped diff, commit, merge, and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented CLI-only fixed human explanations, retaining delivery JSON values and receipt fields. Notify errors are rendered centrally in internal/cli/cli.go, so that file replaces the planned notify.go edit; regression tests extend existing CLI test files. Verification: go test ./internal/cli and task ci passed (security, vet, Staticcheck, unit, race, built-binary smoke); LSP clean on changed source files. Direct isolated binary runs verified bare send (exit 2), missing route/target/session (exit 3) with exact setting/flag/env guidance, doctor ready/not-ready wording and unchanged JSON missing_executable reason. README/design examples match those runs. Independent scoped review is pending.

Independent reviewer completed the one scoped review with no validated findings. Implementation commit aba0806 passed task check:staged and was fast-forward merged into main. Owned worktree and branch tell-17-human-reasons removed via Worktrunk; matching Herdr workspace closure verified. No remaining blockers or resumable work for this item.
<!-- SECTION:NOTES:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: @pi
created: 2026-10-17 01:49
---
Review: ACs verified against built binary (send receipt plus worded reason, missing_route/target/session name setting+flag+env, doctor ready/not ready with explanations, JSON reasons unchanged). humanReason covers every reason doctor can emit. No defects. Unrelated note: TestSendOutcomes/*timeout (150ms budget) flaked 2/10 under high machine load.
---
<!-- COMMENTS:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added actionable human send/notify failures and ready/not-ready doctor output while preserving JSON and exit semantics. Updated examples and regression tests; task ci, isolated binary checks, and independent review passed. Committed as aba0806, merged locally into main, and cleaned up the owned worktree.
<!-- SECTION:FINAL_SUMMARY:END -->
