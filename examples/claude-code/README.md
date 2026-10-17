# Claude Code: know when it needs you

Claude Code stalls on a permission prompt while you are in another window, or
finishes a long task and sits idle. [tell-hook.sh](tell-hook.sh) turns Claude
Code hook events into notifications on your configured route.

| Event | Notifies |
| --- | --- |
| `Notification` (permission prompt, waiting for input) | always, as a warning, with Claude Code's own text |
| `Stop` (turn finished) | only if the turn took 60s or more |
| `UserPromptSubmit` | never; records the turn's start time |

```text
Claude Code · api    Claude needs your permission to use Bash
Claude Code · api    Finished after 412s; waiting for your next prompt
```

## Install

```sh
mkdir -p ~/.claude/hooks
cp examples/claude-code/tell-hook.sh ~/.claude/hooks/
chmod +x ~/.claude/hooks/tell-hook.sh
```

Merge [settings.json](settings.json) into `~/.claude/settings.json` (all
projects) or a project's `.claude/settings.json`. Requires `jq`.

Set a visible route in `~/.config/tell/config.yaml`, such as `desktop`, or
`herdr` to see it in your Herdr session. The hook discards Tell's output, so
the stderr route shows nothing.

```sh
export TELL_CLAUDE_MIN_SECONDS=120   # quieter: only turns of 2+ minutes
```

## Notes

- `-C "$cwd"` selects the session's project, so its shared `tell.yaml`
  (for example an Apprise tag) applies.
- The hook always exits 0, so notification problems never block Claude.
- Away from the desk? Route through [command-routes](../command-routes/README.md)
  `tell-route.sh` to send warnings to your phone.
- To let Claude decide when to notify, or message other agents, use the
  [MCP server](../mcp/README.md) instead of, or alongside, hooks.
