# PR review queue: first-pass reviews by an agent

When someone requests your review, a reviewer agent does a first pass before
you look. [dispatch-reviews.sh](dispatch-reviews.sh) polls GitHub, sends each
new PR to the agent once, and asks you to check anything ambiguous.

```text
[Tell automation: review-queue; route: hcom; intent: request]
Review https://github.com/acme/api/pull/131 (read it with gh pr view/diff).
Report blockers and questions to me. Do not approve, merge, push, or comment on GitHub.
```

## Setup

```yaml
# ~/.config/tell/config.yaml (the scheduler's user)
version: 1
notify:
  via: desktop
send:
  via: hcom
  to: reviewer
  from: review-queue
```

Schedule it every 15 minutes with [noon.yaml](noon.yaml), launchd, or a
systemd timer (see [scheduled-checks](../scheduled-checks/README.md) for
service setup). These never start a run while the previous one is still
going; overlapping runs could send the same PR twice. The scheduler needs
`gh` authenticated and on `PATH`.

## Delivery outcomes drive the state

| `tell send` | Recorded as seen? | Then |
| --- | --- | --- |
| 0 accepted | yes | done |
| 3 not_sent (agent absent, busy, blocked) | no | next run tries again |
| 4 failed, 5 uncertain | yes | you get a warning to check by hand |

Tell does not deduplicate; the `seen` file in `~/.local/state/tell-pr-reviews/`
does.

## Safety

- **Only the URL is sent.** PR titles and bodies are attacker-controllable
  text. The agent fetches them with `gh` and treats them as data.
- **The agent may not act on GitHub.** The instruction limits it to reporting
  to you. Enforce that with the agent's own permissions too, for example a
  read-only token.
