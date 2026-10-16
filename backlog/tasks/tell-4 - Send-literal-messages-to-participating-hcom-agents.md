---
id: TELL-4
title: Send messages to an existing agent through hcom or Herdr
status: Done
assignee:
  - '@brett'
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 22:54'
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
  - ../bump/docs/agents.md
type: feature
ordinal: 5500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: people, scripts, and scheduled jobs need to hand information or instructions to an agent that already has context, for example "nightly tests failed, run 42; see noon logs 42" or, on a weekday schedule, "triage new PRs and report blockers". Scripts should not know which transport reaches that agent, and automation needs safe defaults that raw transport CLIs do not provide. Messaging an agent is a different intent from notifying a person, so it is a separate send command with its own routes, mirroring notify. See the send paragraph in docs/design.md.

Instructions versus data: the body is the sender's own text (information by default, an instruction when the caller asks for a request). Instructions are legitimate when a person or their scheduled job authors them. Untrusted material (job output, logs, PR text) is never embedded; messages carry pointers such as run IDs, paths, and URLs, and the agent reads the data with its own tools.

Scheduled instructions: the caller's scheduler decides when and owns missed runs, overlap, and history; Tell reads no scheduler-specific environment. In the Noon example, a job whose command is tell send delivers at the scheduled time, and a non-zero send fails that run so the user's on_failure hook can notify a person (for example, the agent was not running). When completion must be tracked or the work needs a fresh context, use the scheduler's agent jobs (such as Noon's) instead of send.

Transports (verify against installed versions):
- hcom (src/commands/send.rs) is a mailbox delivered on the agent's next hook, so it never interrupts a turn; the agent must participate in hcom. Syntax: hcom send @NAME -- MESSAGE. No @target broadcasts to everyone; @tag- and several @targets fan out. Without --, --stdin, --file, or --base64 it reads stdin whenever stdin is not a terminal. --from NAME (or -b) sets an external sender. --intent request|inform|ack; request creates reply obligations. --json returns a receipt. Exit 1 on errors, including no matching active participant.
- Herdr: herdr agent prompt NAME TEXT types a new user turn into any agent pane (pi, Claude, Codex). It rejects an agent blocked on an approval or question dialog with agent_blocked. Typing into a working agent would steer its current task, so Tell checks state first (herdr agent get) and delivers only to an idle or done agent. herdr agent prompt has no idle-only option, so an agent can start working between the check and the prompt; document that race and point to hcom for non-interrupting delivery. Herdr documents that a timeout does not prove the prompt was not delivered.
- Unharness (uhh send) is a likely future transport; do not build it here.

Safe defaults Tell owns: exactly one recipient from personal config per project; an explicit sender identity marking automation; inform intent unless the caller requests otherwise; a short provenance header naming the automated sender and route; outbound normalization and secret redaction; never reading stdin implicitly; a one-line receipt (no body) on stderr so Noon run logs record what automation told agents. Setting names (for example send.via and a recipient) are decided here; replace the bootstrap hcom.to setting rather than keeping both.

Scope: send command, docs/design.md boundary diagram and Outcomes extended to send (including reasons busy and blocked), hcom and Herdr routes in internal/delivery reusing TELL-2's subprocess helper, personal-config settings, doctor readiness checks for both routes, local fixture executables, tests, docs, and a scheduled-instruction example.

Non-goals: scheduling in Tell; spawning or resuming agents; broadcast or fan-out; embedding job output; queues; waiting for replies or turns; completion tracking; automatic retries; Unharness.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 task test exercises tell send over hcom and Herdr routes with a literal message and with explicitly selected stdin; local fixtures observe exactly one recipient, the sender identity, hcom intent inform by default and request when the caller asks, and the body as one literal argv element (after -- for hcom), preserving whitespace, quotes, and leading dashes.
- [x] #2 Every message starts with a short provenance header naming the automated sender and route; docs show the exact header and explain that messages carry pointers, never raw job output.
- [x] #3 The Herdr route types nothing into an agent that is working or blocked and returns not_sent with reason busy or blocked; missing, empty, multi-target, and fan-out recipients and absent executables are also not_sent; provider failure is failed; timeout and cancellation are uncertain; nothing is retried, and providers never inherit Tell's stdin.
- [x] #4 Each send writes one receipt line to stderr with route, recipient, intent, and outcome but never the body; --json carries the same fields.
- [x] #5 Recipient and route settings are personal-only, hcom.to is replaced, and tell doctor reports readiness for both routes without sending.
- [x] #6 An example Noon job sends a scheduled instruction with missed: skip, so an instruction is never delivered late after sleep, and cites NOON_RUN_ID as its pointer; docs explain Noon's ownership of timing, how a failed send reaches a person through on_failure, mailbox versus typed-turn semantics, that dispatch is not completion, and when to use a Noon agent job instead; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add personal send.via/to/from configuration (replace hcom.to), default inform intent, and body-free send receipts.
2. Reuse bounded subprocess execution for hcom single-target mailbox delivery and Herdr idle/done-only prompts; reuse those read-only probes in doctor.
3. Add fixture-based public CLI regression tests for literal/stdin boundaries, unsafe targets, state gates, outcomes, preparation, and readiness.
4. Document exact provenance/transport contracts and a Noon scheduled instruction; run task ci and one scoped independent verification, commit, merge locally, finalize backlog, and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Claim d164d29e5915e1eb3a0632e8f2082f6e acquired by loop dcf00e7e-de08-4cea-b372-071c3a2f2f78. Worktree .worktrees/tell-4-send, creation receipt .git/worktrees/tell-4-send/agent-creation.json. Verified Herdr 0.9.5 get JSON: result.agent.agent_status; missing agent error agent_not_found. hcom not installed; verified upstream send.rs/list.rs at 669a06703aad3b503d6eac0ce9fd744bf3f7c771 (exact local names, tag-ending-hyphen fanout, explicit --from/--intent/-- message).

Implementation in .worktrees/tell-4-send: send.via/to/from (personal sources), hcom exact active-recipient probe and explicit -- message argv, Herdr idle/done gate and resolved pane prompt, shared bounded process helper, body-free receipts, send doctor readiness, provenance/preparation, and scheduled-send.yaml. Verification: mise exec -- task ci PASS (unit, race, smoke, vet, Staticcheck, Gitleaks, govulncheck); LSP clean on send CLI/delivery and configuration. Noon bin/noon --config examples/noon/scheduled-send.yaml --project WORKTREE config check PASS (1 job). Independent verifier 70d8c175-7ee9-4af7-9814-ff39c3669987 running, read-only; delivery/merge pending.

Acceptance evidence: AC1/2/4 TestSendLiteralAndStdin passes for both routes, literal/explicit stdin and inform/request, exact single-recipient argv/header, body-free one-line stderr and JSON metadata. AC3 TestSendRefusesUnsafeTargets and TestSendOutcomes pass: unsafe/absent targets, working/blocked/unknown states, missing executables, failed providers, post-start timeout/cancellation, no retries or inherited stdin. AC5 TestPersonalRoutingSources/Overrides and TestSendReadinessAndPreparation pass for personal routing, read-only doctor, normalization and redaction. AC6 full task ci passes after correction; Noon config check accepts scheduled-send.yaml; isolated /tmp/tell-scheduled-example-check.py executed both YAML shell commands with a fake Tell, confirming failed dispatch exit propagation, literal NOON_RUN_ID pointer, and separate human failure notification. Docs cover missed: skip, timing ownership, transport semantics/race, dispatch versus completion, and Noon agent jobs.
Single independent verification pass found hcom unknown states treated as active. Fixed with confirmed active/listening/blocked allowlist (upstream shared/constants.rs); extended public CLI regressions proving unknown/error/launching/inactive states neither send nor report ready. Full task ci rerun PASS; affected LSP clean. No further general review performed.
Delivery: implementation commit 6296b134b143d6f24bbb89d328f76c9e1317460d fast-forward merged into main after rebasing over concurrent backlog-only commit 2da4b7b; preserved that change. task check:staged passed. Owned worktree/branch tell-4-send removed via Worktrunk; exact Herdr workspace wE6 confirmed absent. No retained checkout/workspace, blocker, or resumable implementation step. No push.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented safe single-recipient tell send through hcom and Herdr, personal configuration, provenance/redaction, receipts, readiness, and scheduled instructions. Verified full task ci, fixture-based safety/outcome regressions, and Noon example validation/execution. Independent verification finding fixed and checks rerun. Merged 6296b13 into main; owned worktree, branch, and Herdr workspace removed. No push.
<!-- SECTION:FINAL_SUMMARY:END -->
