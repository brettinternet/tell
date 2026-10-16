---
id: TELL-4
title: Send messages to an existing agent through hcom or Herdr
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 21:43'
labels: []
dependencies:
  - TELL-2
  - TELL-7
  - TELL-10
  - TELL-12
references:
  - 'https://github.com/aannoo/hcom'
documentation:
  - docs/design.md
  - ../unharness/docs/architecture.md
type: feature
ordinal: 5500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: humans, scripts, and Noon hooks sometimes need to hand something to an agent that already has context, for example "nightly tests failed, run 42; see noon logs 42". Scripts should not know which transport reaches that agent, and automation needs safe defaults that raw transport CLIs do not provide. Messaging an agent is a different intent from notifying a person, so it is a separate send command with its own routes, mirroring notify.

Build this when a real caller exists: a Noon hook or review-loop script that needs to message a specific agent. It follows the Noon hook example (TELL-5) so the outcome contract and hook pattern are proven first.

Transports (verify against installed versions):
- hcom (src/commands/send.rs) is a mailbox delivered on the agent's next hook; the agent must participate in hcom. Syntax: hcom send @NAME -- MESSAGE. No @target broadcasts to everyone; @tag- and several @targets fan out. Without --, --stdin, --file, or --base64 it reads stdin whenever stdin is not a terminal. --from NAME (or -b) sets an external sender. --intent request|inform|ack; request creates reply obligations. --json returns a receipt. Exit 1 on errors, including no matching active participant.
- Herdr: herdr agent prompt NAME TEXT types a new user turn into any agent pane (pi, Claude, Codex). It rejects an agent blocked on an approval or question dialog with agent_blocked before sending input. Herdr documents that a timeout does not prove the prompt was not delivered. It is more intrusive than a mailbox.
- Unharness (uhh send) is a likely future transport; do not build it here.

Safe defaults Tell owns: exactly one recipient, looked up from personal config per project; an explicit sender identity marking automation; inform intent unless the caller explicitly requests otherwise; a short provenance header stating the message is automated and its content is data, not instructions (Unharness applies the same untrusted-data rule); outbound normalization and secret redaction; never reading stdin implicitly. Setting names (for example send.via and a recipient) are decided here; replace the bootstrap hcom.to setting rather than keeping both.

Scope: send command, hcom and Herdr routes in internal/delivery reusing TELL-2's subprocess helper, personal-config settings, doctor readiness checks for both routes, local fixture executables, tests, docs, examples.

Non-goals: spawning or resuming agents; broadcast or fan-out; queues; waiting for replies or turns; completion tracking (Noon agent jobs own new work with completion); automatic retries; Unharness.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test exercises tell send over hcom and Herdr routes with a literal message and with explicitly selected stdin; local fixtures observe exactly one recipient, the sender identity, hcom's default inform intent, and the body as a literal argv element (after -- for hcom), preserving whitespace, quotes, and leading dashes.
- [ ] #2 Every message carries a short provenance header identifying it as automated and its body as data, not instructions; docs show the exact header.
- [ ] #3 Missing, empty, multi-target, and fan-out recipients and absent executables return not_sent before any provider runs, as does Herdr agent_blocked; provider failure is failed; timeout and cancellation are uncertain; nothing is retried or reported as completed agent work, and providers never inherit Tell's stdin.
- [ ] #4 Recipient and route settings are personal-only, hcom.to is replaced, and tell doctor reports readiness for both routes without sending.
- [ ] #5 task ci passes; help and docs explain mailbox versus typed-turn semantics, hcom participation, explicit target and sender selection, that dispatch is not completion, and when to use a Noon agent job instead.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
