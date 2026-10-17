---
id: DRAFT-4
title: Receive signed provider webhooks on a loopback HTTP listener
status: Draft
assignee: []
created_date: '2026-10-17 21:02'
labels:
  - listen
dependencies: []
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context
TELL-26 adds Slack over Socket Mode, which needs no public URL. Many other providers deliver only signed HTTP webhooks (GitHub, Linear, Sentry, the Slack Events API). On a developer machine those arrive through a forwarder such as `gh webhook forward` or a tunnel. A loopback HTTP listener inside `tell listen` would let each provider become a source that emits the TELL-26 envelope and reuses TELL-27 routes unchanged (decision-1, item 7).

## Candidate scope
An optional HTTP listener for `tell listen`, loopback by default, with one built-in source per provider. Each source settles its own signature scheme, replay window, handshake (for example Slack `url_verification`), delivery id for deduplication, and which headers, if any, enter `data`. Secrets come from `*_env` settings and are removed from child environments.

## Questions for refinement
- Which provider comes second, and through which forwarder? Promote only for a concrete one.
- One address with a path per source, or one listener per source?
- Refuse non-loopback binds, or allow them with a warning?
- Body size, header, and timeout limits; respond 2xx after verification and before routing?
- Is a generic HMAC source worth having, or does every source stay provider-specific?
- How do forwarder restarts and provider retries interact with the 10-minute dedup window?

## Candidate acceptance for refinement
- Unsigned, wrongly signed, and expired requests are rejected and never routed.
- Verified deliveries emit one envelope with the provider delivery id as `id`.
- Tests use only local requests; no real provider or forwarder.
- Inspection never starts the listener.

## Boundaries and references
No TLS, public exposure, durable queue, or retries. Depends on TELL-26 and TELL-27. See ../noon/docs/webhooks.md for prior art on signed webhooks; do not move Noon webhooks into Tell.

Draft idea only. Refine before promotion or implementation.
<!-- SECTION:DESCRIPTION:END -->
