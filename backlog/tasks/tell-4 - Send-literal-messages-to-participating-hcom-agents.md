---
id: TELL-4
title: Send literal messages to participating hcom agents
status: To Do
assignee: []
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 20:47'
labels: []
dependencies:
  - TELL-2
references:
  - 'https://github.com/aannoo/hcom'
documentation:
  - docs/design.md
type: feature
ordinal: 4000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: humans and scheduled scripts need to message an existing hcom participant from another shell without coupling the sender to agent lifecycle management. Messaging an agent is a different intent from notifying a person, so it is a separate send command.

Context (hcom src/commands/send.rs):
- Syntax: hcom send @name -- <message>. Recipients are positional @targets; hcom has no --to flag.
- No @target broadcasts to every agent; @tag- (trailing dash) fans out to every agent with that tag; several @targets fan out.
- Without --, --stdin, --file, or --base64, hcom reads the message from stdin whenever stdin is not a terminal.
- --from <name> (or -b for bigboss) sets an external sender; otherwise identity may come from the caller's agent environment.
- hcom exits 1 on errors, including when no active participant matches.
- Tell's root --to flag and TELL_HCOM_TO already map to hcom.to with flag > env > file precedence.

Scope: send command, the hcom route in internal/delivery reusing TELL-2's subprocess helper, exercised config validation, local fixture executables, tests, docs, and examples.

Non-goals: spawning or resuming agents, broadcast or fan-out, queues, waiting for replies, completion tracking, automatic retries.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test exercises tell send with a literal message and with explicitly selected stdin; a local hcom fixture observes exactly one @recipient, an explicit sender identity, and the body after -- as separate argv elements, preserving whitespace, quotes, and leading dashes.
- [ ] #2 Missing, empty, multi-target, and tag fan-out (trailing -) recipients fail before hcom runs, and hcom never inherits Tell's stdin.
- [ ] #3 Absent executable, provider failure, timeout, and cancellation are distinguishable non-zero outcomes; an uncertain send is never retried or reported as completed agent work.
- [ ] #4 task ci passes; help and docs explain hcom participation, explicit target and sender selection, and that dispatch acknowledgment is not execution completion.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
