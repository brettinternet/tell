# Codex: notify when a turn completes

Codex CLI runs a `notify` program after each agent turn.
[codex-tell.sh](codex-tell.sh) forwards that event to your Tell route, so it
reaches your desktop, Herdr session, or phone instead of only the terminal.

```text
Codex · api    Turn complete; waiting for your next prompt
```

## Install

```sh
cp examples/codex/codex-tell.sh ~/.local/bin/ && chmod +x ~/.local/bin/codex-tell.sh
```

Add a top-level key to `~/.codex/config.toml` (before any `[table]`), using an
absolute path:

```toml
notify = ["/Users/me/.local/bin/codex-tell.sh"]
```

Requires `jq` and a visible personal route such as `notify.via: desktop`.

## Variations

Include a preview of the reply (it goes to your route, so prefer a local one):

```sh
preview=$(printf '%s' "$payload" | jq -r '."last-assistant-message" // "" | split("\n")[0][:120]')
```

To let Codex decide when to notify or message other agents, add the
[MCP server](../mcp/README.md): `codex mcp add tell -- tell mcp`.
