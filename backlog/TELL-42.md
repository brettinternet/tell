---
id: TELL-42
title: Add direct Pushover route with receipts, tags, and callback
status: done
type: feature
labels: [notify, escalation]
related: [TELL-33, TELL-35, TELL-36, TELL-38]
created: 2026-11-02T03:47:01Z
updated: 2026-11-02T04:48:55Z
---

### Context

Escalation is Tell's main advantage. The pattern (docs/design.md, Escalation) is to schedule every tier first and let acknowledgement cancel them, so Tell never waits minutes inside a process. Pushover is the provider the user relies on, and the only one where Tell needs the API response. Neither existing path can supply it. Shoutrrr's `Sender.Send` returns only `error` and discards the response body, and v0.22.1 also drops `priority=2`. Apprise prints no receipt.

A direct route sends once and exposes three things:

- the `receipt`, so a later tier can check it before sending;
- `tags`, so acknowledgement from any source can stop the phone repeating;
- `callback`, so Pushover can push the acknowledgement to `tell listen` (TELL-33).

### API

- `POST https://api.pushover.net/1/messages.json` with `token`, `user`, `message`, `title`, `priority`, and `device`. Emergency priority (`2`) also needs `retry` (at least 30s) and `expire` (at most 10800s), and may take `tags` and `callback`. The response includes `receipt`.
- `GET /1/receipts/{receipt}.json?token=` returns `acknowledged`, `expired`, and related fields. Poll no more than once every 5s; a receipt stays available for a week.
- `POST /1/receipts/cancel_by_tag/{tag}.json` with `token` cancels the app's active emergency messages that carry the tag.

### Scope

- Add a `pushover` route usable in `--via` and in cascades. Configure it with a `pushover: {token_env, user_env, priority, retry, expire, device}` step or top-level `pushover.*`.
- Credentials come only through env var names in personal config, like Shoutrrr `urls_env`, and are refused in shared `tell.yaml`. This is a documented exception to keeping outbound credentials with the provider: Tell needs the response.
- Accept per-call `--pushover-tag TAG` and `--pushover-callback URL` flags, because both are per incident. Values are literal: the caller (`raise.sh`) derives the tag from its incident hash. Tell doesn't hash.
- Send once. An emergency result includes `receipt` in `--json`. The receipt is data on stdout, never in diagnostics.
- Add `tell pushover status RECEIPT` (prints acknowledged/expired; exit 0 when acknowledged, 3 when not) and `tell pushover cancel --tag TAG` for tier guards and `ack.sh`. Both are one-shot, with no waiting or polling loop in Tell. Bare `tell pushover` only inspects, showing help and readiness.
- Never print the token, user key, receipt, callback URL, or body in diagnostics.
- Add doctor support: the env vars are set and well-formed, with no network calls.
- Update examples/escalate to use the direct route as the Pushover tier. Tier guards check the receipt, and `ack.sh` cancels by tag.

### Outcome mapping

| Result | Outcome | Exit |
| --- | --- | --- |
| HTTP 200, status 1 | `accepted` (plus `receipt` for emergency) | 0 |
| credentials missing or rejected (4xx) | `not_sent/invalid_config` | 3 |
| 5xx | `failed/provider_failure` | 4 |
| timeout or cancellation after the request starts | `uncertain` | 5 |

### Non-goals

In-process acknowledgement waits or polling loops, receiving the callback (TELL-33), attachments, Pushover Glances or Teams, state kept between invocations, and any call to or knowledge of a scheduler.

### Dependencies

None to start. TELL-33 consumes `callback`. Related: TELL-35, TELL-36, TELL-38, decision-1.

### Files

`internal/delivery/` (new `pushover.go` using net/http, doctor, cascade wiring), `internal/config/`, `internal/cli/`, `tell.schema.json`, README route table, docs/configuration.md, docs/design.md (Pushover delivery, Escalation), examples/escalate.

## Acceptance criteria

- [x] An emergency pushover step delivers to a real Pushover account and --json includes the receipt; normal priorities return accepted without one
  Evidence: 5f0a11f. Live account 2026-11-01: tell --config emergency.yaml --json notify --pushover-tag tell42-ack -> exit 0 {outcome:accepted, receipt:r2kf…}; priority normal -> exit 0 accepted, no receipt field; doctor ready.
- [x] Cancel-by-tag stops a repeating emergency alert and receipt status reports acknowledged after acking in the app, each as a one-shot command; verified with a real account
  Evidence: Live: pushover status before ack -> 'not acknowledged' exit 3; after user acked in app -> {acknowledged:true} exit 0. Second emergency (tag tell42-cancel): pushover cancel --tag -> accepted exit 0; status then {acknowledged:false, expired:true} exit 3 (repeats stopped). Each a one-shot command.
- [x] Token, user key, receipt, and body never appear in diagnostics or doctor output; credentials in shared tell.yaml are refused; covered by tests
  Evidence: internal/delivery/pushover_test.go assertNoPushoverLeak over all outcome cases; internal/cli/pushover_test.go TestPushoverInspectionNeverLeaksCredentials (doctor json/human + bare tell pushover) and TestPushoverConfigValidation (shared tell.yaml pushover.* refused). Reviewer pass found no leak paths. task ci passed.
- [x] Request building (emergency retry/expire/tags/callback) and response mapping are covered by tests against an httptest server; Tell has no polling loop or wait
  Evidence: TestPushoverNotifyRequest (emergency retry/expire/tags/callback/device form fields, receipt; non-emergency omits them), TestPushoverNotifyOutcomes (400/429/500/malformed/no-receipt/slow->uncertain/missing creds/unreachable), TestPushoverStatusAndCancel against httptest. No polling/wait: status and cancel are single requests.
- [x] examples/escalate uses the direct route as the Pushover tier, with a receipt guard in later tiers and ack.sh cancelling by tag; docs explain why Shoutrrr/Apprise cannot expose receipts; task ci passes
  Evidence: examples/escalate: config.yaml direct pushover emergency; raise.sh sends tier 1 with --pushover-tag inc-<hash>, saves receipt per raise; tier.sh guards later tiers with tell pushover status; ack.sh cancels by tag + Noon jobs. README + docs/design.md explain Shoutrrr Send returns only errors and Apprise CLI prints no provider response. shellcheck clean; task ci passed after rebase on 870b871.

## Notes

Worktree: .worktrees/feat-tell-42-pushover branch feat/tell-42-pushover session fa2b28fa-f05a-48d4-b25d-9669ae3f4f44
