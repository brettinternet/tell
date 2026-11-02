---
id: TELL-35
title: Document provider-native escalation with Pushover emergency priority
status: done
type: docs
labels: [notify, escalation]
created: 2026-11-02T03:18:12Z
updated: 2026-11-02T03:29:32Z
---

## Context

TELL-33 explored acknowledgement-driven escalation. For one person whose alerts should get louder until acknowledged, the cheapest path is provider-native: Pushover emergency priority repeats the alert at a set retry interval until the user acknowledges it in the Pushover app, or until it expires. Apprise already supports this through `pover://` URL parameters (`priority=emergency`, `retry`, `expire`). It needs no new Tell code: it is one Apprise step, usually the last step of a TELL-30 cascade.

## Scope

Document and demonstrate provider-native escalation with an Apprise step that selects a Pushover emergency destination by tag, config, or urls_env. Explain that `accepted` means Pushover accepted the alert, not that the user acknowledged it; acknowledgement and repeats happen inside Pushover. Mention that PagerDuty or Opsgenie through Apprise follow the same pattern without describing them in detail.

## Non-goals

Tell code changes; polling Pushover receipts; Pushover credentials or URLs in Tell config or argv; timers handled by Tell or Noon (see the Noon-timed tiers task); remote acknowledgement handled by Tell (TELL-33).

## Dependencies

None. Builds on TELL-30 (done).

## Modified-file boundaries

examples/escalate/** (new), examples/README.md, docs/design.md (a short escalation section linking the example), README.md (one line in the notify fallback section if needed).

## Verification

Check the Pushover parameters against Apprise's pover plugin source or wiki for the pinned Apprise version. If credentials are available, send one real emergency alert, then acknowledge it; otherwise run `apprise --dry-run` against the example config. Run `task ci` for doc and link checks.

## Acceptance criteria

- [x] examples/escalate/README.md shows an Apprise step selecting a Pushover emergency destination (priority=emergency with retry and expire) as the last step of a notify cascade, with no URLs or credentials in Tell config
  Evidence: examples/escalate/README.md + config.yaml: cascade [apprise:{tag: oncall}, apprise:{tag: escalate}], last step selects pover://...?priority=emergency&interval=120&expire=3600 held only in Apprise's escalate.yaml. tell --config critical.yaml config check: valid; doctor: apprise:escalate ready, services: 1. Commit b5f258f.
- [x] Docs state that accepted means Pushover accepted the alert, not that the user acknowledged it, and that repeats and acknowledgement happen inside Pushover
  Evidence: README 'Accepted is not acknowledged' section and docs/design.md#escalation state accepted = Pushover accepted, repeats/ack happen inside Pushover, Tell does not poll receipts. Commit b5f258f.
- [x] The Pushover parameters are verified against the pinned Apprise version (source or a dry run), and the evidence is recorded
  Evidence: Verified against Apprise 2.1.0 (PyPI, installed via mise) apprise/plugins/pushover.py: priority=emergency|2; repeat param is 'interval' (min 30, default 900, sent as API 'retry'); expire 0-10800 default 3600. Apprise.instantiate: interval=120&expire=3600 -> (2,120,3600); retry=120 ignored -> interval 900 (documented). apprise --dry-run --tag escalate on example: 1 service, exit 0; interval=10 -> URL dropped, exit 3 (documented as not_sent/no_match). No credentials, so no real send.
- [x] examples/README.md and docs/design.md link the example; task ci passes
  Evidence: examples/README.md routes table row and docs/design.md Escalation section link examples/escalate; README fallback Details links it. task ci passed (vet, staticcheck, tests, race, build, smoke). Merged ff to main b5f258f.

## Summary

Added examples/escalate (README, apprise.yaml, config.yaml): Pushover emergency as final Apprise step of a critical-alert cascade selected with tell --config, preceded by an optional oncall (PagerDuty/Opsgenie) step. Documents interval (not retry), limits, accepted != acknowledged. Linked from examples/README.md, docs/design.md#escalation, README. Verified against Apprise 2.1.0 source and dry runs; task ci passed. Commit b5f258f merged to main; no review findings.
