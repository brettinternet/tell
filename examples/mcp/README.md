# MCP: let agents decide when to tell you

Hooks notify on fixed events. With `tell mcp`, the agent decides: "done, PR
is up", "blocked: which database should the migration target?", or a review
request to another agent. Tell exposes three tools: `notify`, `send`, and
read-only `doctor`.

## Setup

```sh
claude mcp add tell -- tell mcp
codex mcp add tell -- tell mcp
```

Other clients (Cursor, Zed, opencode, and so on) take the same command:

```json
{ "mcpServers": { "tell": { "command": "tell", "args": ["mcp"] } } }
```

Then add [AGENTS-snippet.md](AGENTS-snippet.md) to your project's `AGENTS.md`
or `CLAUDE.md` so agents know when to call it and what never to include.

## What the agent sends

```json
{ "name": "notify", "arguments": { "type": "failure", "message": "Blocked: migration 0042 drops users.email, which billing reads. Keep or drop? Branch feat/email-cleanup" } }
{ "name": "send", "arguments": { "intent": "request", "message": "Review feat/email-cleanup at 3f9c2e1; blockers first." } }
```

By default the agent cannot pick a route or recipient. Messages use your
personal config: desktop banner for `notify`, `reviewer` over hcom for `send`.

## Letting agents choose a route

[config.yaml](config.yaml) sets `mcp.agent_targets: true`. Agents may then
choose among routes and targets you configured:

```json
{ "name": "notify", "arguments": { "via": "apprise", "apprise_tag": "phone", "type": "failure", "message": "Prod deploy blocked on approval" } }
{ "name": "send", "arguments": { "via": "herdr", "to": "tests", "intent": "inform", "message": "Schema changed; rerun fixtures" } }
```

They still cannot set the command executable, Apprise config file, sender
name, timeout, or secret policy. Only enable this in personal config, never a
shared repository file. Revoke it for one client with
`tell mcp --mcp-agent-targets=false`.

## Notes

- Results are body-free: route, outcome, and reason. Provider output never
  reaches the agent.
- `accepted` means dispatched, not read. The snippet tells agents not to
  resend accepted, failed, or uncertain messages.
- For agent-to-agent workflows outside MCP, see
  [agent-handoff](../agent-handoff/README.md).
