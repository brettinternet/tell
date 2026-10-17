---
id: DRAFT-3
title: Route matched events to notify and send without a wrapper command
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
TELL-27 routes run commands. A route that only pings a person or an agent needs a wrapper script that calls `tell notify` or `tell send`. Built-in route actions could deliver through the configured personal routes in-process, with the normal message preparation (normalization, secret redaction, truncation) and the usual outcomes.

## Candidate scope
Route actions `notify` (message, optional title and type) and `send` (message, optional intent) whose text can reference envelope fields. Routing still comes only from personal configuration.

## Questions for refinement
- Are wrapper commands enough in practice? Promote only after the same wrapper pattern repeats.
- Template syntax (Go text/template subset or simple `{event.channel.name}` placeholders), and behavior for missing fields.
- Event text comes from outside (for example, any Slack user who can name a channel). Should `send` allow only `--intent inform`, or only pointers such as channel ids and links rather than raw text, given the existing guidance to send pointers rather than untrusted content?
- Must templates be prevented from choosing routes, recipients, or argv? (Expected answer: yes.)
- Should a route allow a command action and a notify or send action together?

## Candidate acceptance for refinement
- A matched event delivers one notification or message through the configured route, with outcomes logged like command actions.
- Event-derived text passes the same preparation pipeline as CLI input.
- Event data cannot change routes, recipients, or executables.

## Boundaries and references
No fan-out beyond the existing one-route-per-action rule, no retries, no waiting for replies. Depends on TELL-27. Follows decision-1 and the Agent delivery section of docs/design.md.

Draft idea only. Refine before promotion or implementation.
<!-- SECTION:DESCRIPTION:END -->
