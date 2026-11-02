---
id: TELL-33
title: Explore remote acknowledgement for escalation via tell listen
status: draft
labels: [notify, escalation]
blocked_by: [TELL-42]
related: [TELL-26, TELL-27, TELL-29, TELL-35, TELL-36, TELL-38]
rank: af878787878787867
created: 2026-10-20T19:35:00Z
updated: 2026-11-02T04:02:46Z
---

### Context

Escalation schedules every tier up front and lets acknowledgement cancel them (docs/design.md, Escalation). Local acknowledgement already works: `ack.sh`, or `tell notify --via alert … && ack.sh KEY` (TELL-38). What remains is remote acknowledgement, meaning stopping pending tiers from a phone with no terminal.

Tell stays stateless and scheduler-agnostic (decision-1). A `tell listen` source receives the acknowledgement event, and a route action runs a user command (`ack.sh KEY`). Tell never calls or reads a scheduler. That command decides what to cancel: scheduler jobs, and the Pushover tag through TELL-42.

### First channel: Pushover callback

An emergency message sent with `callback=URL` (TELL-42) makes Pushover POST `receipt`, `acknowledged`, `acknowledged_at`, `acknowledged_by`, and `acknowledged_by_device` to that URL when someone acknowledges it. Pushover retries after a minute unless it gets a 2xx. This is an inbound webhook, so it goes through the TELL-29 signed ingress (for example cloudflared) with no polling.

Pushover does not sign callbacks, so the URL carries the authentication. Put an HMAC of the incident key, made with a personal secret held in an env var, in the path. The listener recomputes it and rejects mismatches. The incident key is the same hash `raise.sh` uses, so no message body leaks into the URL.

Without public ingress, later tiers check the receipt just before sending (TELL-42 status command) and skip themselves after an acknowledgement. Acknowledgement then lands at most one tier late. Document this as the fallback, not as part of this task.

### Later channels

Slack reactions on the alert message (TELL-26 Socket Mode already receives events), and ntfy action buttons through the same webhook ingress. Slack interactive buttons would need new interactivity ingress.

### Questions for refinement

- The exact route config for a webhook source that validates a path HMAC, and how this fits TELL-29's signing model.
- Which listener diagnostics log rejected acknowledgements without bodies or URLs.
- Whether the action receives the incident key as an argument or through the environment.

### Candidate acceptance

- A Pushover acknowledgement reaches `tell listen` through ingress and runs the action with the incident key, within the action timeout. The scheduler's pending tiers are cancelled.
- A callback with a missing or wrong HMAC is rejected, runs nothing, and is logged without secrets, receipts, or bodies.
- Tell holds no state: restarting the listener loses no pending tiers, and Pushover's retry delivers an acknowledgement that arrived while the listener was down.
- Tell code never invokes or reads a scheduler. Examples may use Noon.

### Non-goals

Multi-person on-call, quiet hours, a policy schema in Tell, durable state in Tell, and polling inside Tell.

### Dependencies

TELL-42 (callback and tags). Builds on TELL-29 (its cloudflared ingress verifies GitHub signatures only, so a Pushover callback source is new) and TELL-27 (route/action model). Related: TELL-26, TELL-36, TELL-38, decision-1.
