# CI → agent: hand red builds to a fixer agent

You push, CI fails ten minutes later, and you are already on something else.
[watch-pr.sh](watch-pr.sh) waits for the PR's checks. When they fail it asks a
fixer agent to investigate and tells you whether the agent got the request.

```sh
git push && examples/ci-to-agent/watch-pr.sh &
```

The agent receives pointers, not logs:

```text
[Tell automation: ci-watch; route: hcom; intent: request]
CI failed on feat/rate-limit at 3f9c2e1 (https://github.com/acme/api/pull/128).
Failing checks:
- test (ubuntu-latest): https://github.com/acme/api/actions/runs/123/job/456
Read logs with: gh run view <run-id> --log-failed
Fix on feat/rate-limit and push. Do not rerun checks without a change.
```

You receive:

```text
CI failed: feat/rate-limit
https://github.com/acme/api/pull/128
Asked the fixer agent to investigate.
```

## Setup

```yaml
# ~/.config/tell/config.yaml
version: 1
notify:
  via: desktop
send:
  via: hcom
  to: fixer
  from: ci-watch
```

Start the fixer agent in a checkout of the branch, with `gh` authenticated.
Run `tell doctor` to confirm both routes.

## Why pointers

CI logs contain untrusted text: test names, dependency output, PR content.
Pasting them into an agent's prompt turns that text into instructions. The
agent reads logs with its own tools, where it treats them as data. Tell's
provenance header marks the message as automation, not as you.

## Outcomes

| `tell send` | Your notification says | Resend? |
| --- | --- | --- |
| 0 accepted | asked the fixer agent | no |
| 3 not_sent (agent missing, busy, blocked) | not reached | yes, once it is available |
| 4 failed, 5 uncertain | check before resending | only after checking |
