---
id: TELL-40
title: Map Herdr notification show reasons to outcomes
status: done
type: feature
labels: [notify, escalation]
created: 2026-11-02T03:42:32Z
updated: 2026-11-02T03:57:25Z
---

## Context

`herdr notification show` prints a JSON result whose `reason` is one of `shown`, `disabled`, `rate_limited`, `no_foreground_client`, or `busy` (Herdr API `NotificationShowReason`, protocol 22). Tell's Herdr route (internal/delivery/herdr.go) only checks the exit code, so every reason becomes `accepted` and stops the cascade, even when Herdr didn't show anything.

Observed with Herdr 0.9.5: a session with no attached client still returned `{"result":{"reason":"shown","shown":true,"type":"notification_show"}}`. So this mapping does not yet detect detached sessions. That needs the upstream Herdr change in the companion task. It does fix cases where notifications are disabled or rate limited, and it picks up detached-session detection automatically once Herdr reports it.

## Scope

- Capture Herdr's stdout with a bounded buffer through the existing `process` helper in internal/delivery/delivery.go, and parse `result.reason` and `result.shown`.
- Map `shown` to `accepted`. Map `disabled`, `no_foreground_client`, `rate_limited`, and `busy` to `not_sent` with Herdr's reason name. Map `shown:false` with an unknown reason to `not_sent/not_shown`. If the output can't be parsed, keep the current exit-code behavior.
- Keep the existing argv guards, `HERDR_*` stripping, and `--sound none`.

## Non-goals

Detecting attached clients by inspecting processes, reusing TELL-37's local screen-lock check for Herdr (clients may be attached remotely over SSH), retries after rate_limited or busy, changes to Herdr, new config keys.

## Dependencies

None.

## Modified-file boundaries

internal/delivery/herdr.go, internal/delivery tests for Herdr (extend existing ones), internal/cli/herdr_test.go only if outcomes visible to the CLI change, docs/design.md (Herdr delivery), README.md only if the outcome tables there list Herdr reasons.

## Verification

Use captured Herdr 0.9.5 output in a parser test. Run against a real session (`herdr --session SESSION ...`) and record what you observe. Run `task ci`.

## Acceptance criteria

- [x] Herdr shown maps to accepted; disabled, no_foreground_client, rate_limited, and busy map to not_sent with that reason, and the cascade advances; covered by a parser test using captured Herdr 0.9.5 output
  Evidence: internal/delivery/herdr_test.go TestHerdrShowResult: captured Herdr 0.9.5 shown -> accepted; rate_limited, disabled, no_foreground_client, busy -> not_sent/<reason>; go test ./internal/delivery passes. Real cascade [herdr, stderr] advanced to stderr on rate_limited.
- [x] Unparseable or empty output with exit 0 stays accepted; shown:false with an unknown reason maps to not_sent/not_shown; covered by the same test
  Evidence: Same test: empty and unparseable exit-0 output -> accepted; shown:false unknown reason -> not_sent/not_shown; shown:true unknown -> accepted.
- [x] Herdr stdout is read with a bounded buffer and never printed or included in diagnostics
  Evidence: herdr.go captures stdout via process() into probeOutput (1 MiB cap, overflow keeps exit-code meaning); stderr discarded; diagnostic is fixed text "Herdr did not show the notification". Verified CLI human output on rate_limited prints only that line, exit 3.
- [x] Verified against a real Herdr session: a normal notification is accepted, and at least one non-shown reason (for example rate_limited by sending a burst, or disabled through Herdr config) is observed and recorded, or recorded as not reproducible
  Evidence: Herdr 0.9.5, session scratch: first tell notify accepted (exit 0); immediate repeats returned {"reason":"rate_limited","shown":false} and tell reported not_sent/rate_limited (exit 3 single route; cascade advanced to stderr). disabled not exercised.
- [x] docs/design.md Herdr delivery table lists the new reasons and notes that a detached session still reports shown in Herdr 0.9.5; task ci passes
  Evidence: docs/design.md Herdr delivery table lists disabled, rate_limited, no_foreground_client, busy, not_shown and notes detached sessions still report shown in Herdr 0.9.5; reasons list and exit-3 row updated. task ci passed (fmt, vet, staticcheck, test, race, build, smoke, integration). README lists no Herdr reasons; unchanged.

## Summary

Herdr route parses notification show JSON (bounded) and maps non-shown reasons to not_sent so cascades advance. Commit f03d747 on main (fast-forward). Verified with parser test on captured 0.9.5 output, real scratch-session rate_limited burst, and task ci. Review: single focused pass, no defects.
