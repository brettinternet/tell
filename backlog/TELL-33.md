---
id: TELL-33
title: Explore remote acknowledgement for escalation via tell listen
status: draft
labels: [notify, escalation]
blocked_by: [TELL-36]
related: [TELL-35]
rank: af878787878787867
created: 2026-10-20T19:35:00Z
updated: 2026-11-02T03:18:24Z
---

## Context

Escalation is split into three parts. TELL-35 covers provider-native escalation (Pushover emergency priority). TELL-36 covers tiers timed by Noon temporary jobs and acknowledged locally with `noon cancel`; no Noon change is needed, because cancelling by job name replaces the cancel-by-key feature this draft originally asked for. What remains is remote acknowledgement: stopping pending tiers from a phone or chat without a terminal.

Accepted does not mean read, so only an explicit acknowledgement stops escalation. Tell stays stateless (decision-1): a `tell listen` route receives the acknowledgement and runs a short local action (TELL-27 route/action model) that calls the TELL-36 `ack.sh`, or `noon cancel` directly.

## Options to evaluate

- ntfy action button posting to the signed webhook ingress (TELL-29 via cloudflared).
- Slack reaction on the alert message (TELL-26 Socket Mode already receives events).
- Slack interactive button: TELL-26 does not handle interactivity payloads today; adding them is new ingress scope.
- Pushover receipts: polling is a timer, so it would belong to Noon, not Tell.

## Questions for refinement

- Authentication: how is an acknowledgement bound to the incident and to the acknowledger, so that a third party cannot silence an incident? For example, an HMAC of the incident key in the action URL with a personal secret, or a Slack user ID allowlist.
- The incident key must not leak message bodies. Is the hashed name from TELL-36 enough?
- A rejected or mismatched acknowledgement must be logged without bodies. Which existing listener diagnostics cover this?
- Which single channel should come first? Ask the user before promotion.

## Candidate acceptance

- A valid acknowledgement from the chosen channel cancels pending tiers within the action timeout.
- An unauthenticated or mismatched acknowledgement is rejected, cancels nothing, and is logged without bodies.
- Tell holds no state; restarting the listener loses no pending tiers.

## Non-goals

Multi-person on-call, quiet hours, a policy schema in Tell, and durable state in Tell.

## Dependencies

TELL-36 (incident naming and ack.sh); TELL-27 route/action model. Related: TELL-26, TELL-29, decision-1.
