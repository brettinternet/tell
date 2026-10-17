# Agent handoff: implementer → reviewer

Two agents, two terminals: one implements, one reviews. Instead of copying
context between them, the implementer (or you) runs one command that hands the
reviewer a precise pointer.

```console
$ TELL_SEND_TO=reviewer examples/agent-handoff/request-review.sh main
tell send: route="hcom" recipient="reviewer" intent="request" outcome=accepted
```

The reviewer receives:

```text
[Tell automation: implementer; route: hcom; intent: request]
Review feat/rate-limit at 3f9c2e1 against main (7 files).
Read: git -C /Users/me/dev/api/.worktrees/rate-limit diff main...3f9c2e1
Report blockers first, then suggestions. Do not edit that checkout.
```

## Setup

Personal config names the transport and default recipient:

```yaml
# ~/.config/tell/config.yaml
version: 1
send:
  via: hcom       # queued for the reviewer's next hook; never interrupts it
  to: reviewer
  from: implementer
```

The reviewer must be a participating hcom agent named `reviewer`. Check with
`tell doctor`. Let the implementer run it too, from a task:

```yaml
# Taskfile.yml
review:request:
  desc: Ask the reviewer agent to review this branch
  cmd: examples/agent-handoff/request-review.sh {{.CLI_ARGS | default "main"}}
```

## More handoffs

```sh
# Planner → implementer in an idle Herdr pane (types a new turn only if idle).
tell send --send-via herdr --herdr-session work --to implementer \
  --intent request "Implement backlog task API-42. Plan: backlog/tasks/api-42.md"

# Status only, no reply expected.
tell send --to planner "Rate-limit branch merged at 9b1d7aa; API-42 done."

# Reviewer → implementer, with findings in a file, not the message.
tell send --to implementer --intent request \
  "Review of 3f9c2e1: 2 blockers in review/3f9c2e1.md. Fix and request review again."
```

## Notes

- Commit first. A SHA is a stable pointer; a dirty working tree is not.
- `request` creates a reply obligation in hcom; `inform` does not.
- `accepted` means queued, not reviewed. Do not resend on a timeout
  (`uncertain`); check with the reviewer first.
- Herdr typing can steer a busy agent if it starts working between the idle
  check and the prompt. Prefer hcom when interruption matters. See
  [fallback](../fallback/README.md) for trying Herdr, then hcom, then a person.
