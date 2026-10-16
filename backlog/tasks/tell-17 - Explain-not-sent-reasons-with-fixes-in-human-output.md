---
id: TELL-17
title: Explain not-sent reasons with fixes in human output
status: To Do
assignee: []
created_date: '2026-10-16 23:24'
labels: []
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
- [ ] #1 Every non-accepted human send result prints the reason in words after the receipt; bare `tell send` explains that a message argument or - is required.
- [ ] #2 missing_route, missing_target, and missing_session explanations name the setting to configure (send.via, send.to, herdr.session) with its flag and TELL_* environment variable.
- [ ] #3 Human doctor output marks each route ready or not ready and explains each not-ready reason in words (for example that apprise was not found on PATH); --json reason codes are unchanged.
- [ ] #4 README and docs/design.md examples match the new human output; task ci passes.
<!-- AC:END -->
