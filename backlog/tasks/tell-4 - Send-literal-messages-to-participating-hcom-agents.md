---
id: TELL-4
title: Send messages to an existing agent through hcom or Herdr
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 21:55'
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
  - ../bump/docs/hooks.md
type: feature
ordinal: 5500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: people, scripts, and Noon jobs need to hand information or instructions to an agent that already has context, for example "nightly tests failed, run 42; see noon logs 42" or, on a weekday schedule, "triage new PRs and report blockers". Scripts should not know which transport reaches that agent, and automation needs safe defaults that raw transport CLIs do not provide. Messaging an agent is a different intent from notifying a person, so it is a separate send command with its own routes, mirroring notify. See the send paragraph in docs/design.md.

Instructions versus data: the body is the sender's own text (information by default, an instruction when the caller asks for a request). Instructions are legitimate when a person or their scheduled job authors them. Untrusted material (job output, logs, PR text) is never embedded; messages carry pointers such as run IDs, paths, and URLs, and the agent reads the data with its own tools.

Scheduled instructions: Noon decides when. A Noon job whose command is tell send delivers at the scheduled time; Noon owns missed runs, overlap, and history. A non-zero send fails that run, so the user's on_failure hook can notify a person (for example, the agent was not running). When completion must be tracked or the work needs a fresh context, use a Noon agent job instead of send.

Transports (verify against installed versions):
- hcom (src/commands/send.rs) is a mailbox delivered on the agent's next hook, so it never interrupts a turn; the agent must participate in hcom. Syntax: hcom send @NAME -- MESSAGE. No @target broadcasts to everyone; @tag- and several @targets fan out. Without --, --stdin, --file, or --base64 it reads stdin whenever stdin is not a terminal. --from NAME (or -b) sets an external sender. --intent request|inform|ack; request creates reply obligations. --json returns a receipt. Exit 1 on errors, including no matching active participant.
- Herdr: herdr agent prompt NAME TEXT types a new user turn into any agent pane (pi, Claude, Codex). It rejects an agent blocked on an approval or question dialog with agent_blocked. Typing into a working agent would steer its current task, so Tell checks state first (herdr agent get) and delivers only to an idle or done agent. Herdr documents that a timeout does not prove the prompt was not delivered.
- Unharness (uhh send) is a likely future transport; do not build it here.

Safe defaults Tell owns: exactly one recipient from personal config per project; an explicit sender identity marking automation; inform intent unless the caller requests otherwise; a short provenance header naming the automated sender and route; outbound normalization and secret redaction; never reading stdin implicitly; a one-line receipt (no body) on stderr so Noon run logs record what automation told agents. Setting names (for example send.via and a recipient) are decided here; replace the bootstrap hcom.to setting rather than keeping both.

Scope: send command, hcom and Herdr routes in internal/delivery reusing TELL-2's subprocess helper, personal-config settings, doctor readiness checks for both routes, local fixture executables, tests, docs, and a scheduled-instruction example.

Non-goals: scheduling in Tell; spawning or resuming agents; broadcast or fan-out; embedding job output; queues; waiting for replies or turns; completion tracking; automatic retries; Unharness.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test exercises tell send over hcom and Herdr routes with a literal message and with explicitly selected stdin; local fixtures observe exactly one recipient, the sender identity, hcom intent inform by default and request when the caller asks, and the body as one literal argv element (after -- for hcom), preserving whitespace, quotes, and leading dashes.
- [ ] #2 Every message starts with a short provenance header naming the automated sender and route; docs show the exact header and explain that messages carry pointers, never raw job output.
- [ ] #3 The Herdr route types nothing into an agent that is working or blocked and returns not_sent with reason busy or blocked; missing, empty, multi-target, and fan-out recipients and absent executables are also not_sent; provider failure is failed; timeout and cancellation are uncertain; nothing is retried, and providers never inherit Tell's stdin.
- [ ] #4 Each send writes one receipt line to stderr with route, recipient, intent, and outcome but never the body; --json carries the same fields.
- [ ] #5 Recipient and route settings are personal-only, hcom.to is replaced, and tell doctor reports readiness for both routes without sending.
- [ ] #6 An example Noon job sends a scheduled instruction and docs explain Noon's ownership of timing, how a failed send reaches a person through on_failure, mailbox versus typed-turn semantics, that dispatch is not completion, and when to use a Noon agent job instead; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
