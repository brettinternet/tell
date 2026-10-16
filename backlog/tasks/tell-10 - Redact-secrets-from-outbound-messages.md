---
id: TELL-10
title: Redact secrets from outbound messages
status: To Do
assignee: []
created_date: '2026-10-16 21:41'
labels: []
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
- [ ] #1 Before any route runs, the title and body are scanned with an embedded default ruleset (Betterleaks or Gitleaks library; notes record the choice and why). By default each finding is replaced with a fixed marker and delivery continues; the JSON result reports the redaction count, never the secret.
- [ ] #2 A reject policy, settable only in personal config, fails with exit 1 and reason secret_detected before any provider runs; shared tell.yaml cannot weaken the policy.
- [ ] #3 Tests use synthetic tokens matching default rules and cover redact and reject for title and body. Added per-invocation latency and binary size are measured and recorded in notes; task ci, including govulncheck, passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
