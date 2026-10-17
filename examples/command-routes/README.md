# Command routes: your own delivery logic

`notify.via: command` runs one personal executable per notification. It gets
the body on stdin and `TELL_TITLE`, `TELL_TYPE`, and `TELL_PROJECT` in the
environment. Exit 0 means accepted, 3 means nothing was sent, anything else
is failed. See [command delivery](../../docs/design.md#command-delivery).

Every script that calls `tell notify` (hooks, agents, schedulers) then
follows your rules without knowing them.

| Provider | Does |
| --- | --- |
| [tell-route.sh](tell-route.sh) | Router: failures/warnings to phone, the rest to desktop, quiet hours at night |
| [tell-ntfy.sh](tell-ntfy.sh) | Push to a phone with [ntfy](https://ntfy.sh); priority and emoji tag from the type |
| [tell-journal.sh](tell-journal.sh) | Append to a local TSV log: an audit trail of what automations said |
| [tell-tmux.sh](tell-tmux.sh) | Flash in the tmux status line |
| [tell-say.sh](tell-say.sh) | Speak failures aloud (macOS) |

## Install

```sh
mkdir -p ~/.local/bin ~/.config/tell-ntfy
cp examples/command-routes/tell-*.sh ~/.local/bin/
chmod +x ~/.local/bin/tell-*.sh

printf '%s\n' 'https://ntfy.sh/pick-a-long-random-topic' > ~/.config/tell-ntfy/url
# Optional, for protected topics:
# printf '%s\n' 'Authorization: Bearer tk_...' > ~/.config/tell-ntfy/headers
chmod 600 ~/.config/tell-ntfy/*
```

Then point Tell at one provider in `~/.config/tell/config.yaml`
([config.yaml](config.yaml)):

```yaml
version: 1
notify:
  via: command
  command: [/Users/me/.local/bin/tell-route.sh]
```

```sh
tell doctor                                         # checks the executable exists; never runs it
tell notify --type failure "Nightly backup failed"  # → phone
tell notify "Build finished"                        # → desktop, or not_sent at 23:00
```

## Writing your own

- Keep secrets in the provider's own files. `tell-ntfy.sh` passes the auth
  header with `curl -H @file`, so the token never appears in argv.
- `exec` the final program. Tell kills only its direct child on timeout.
- Exit 3 only when you are sure nothing was sent: callers may retry or try
  another route.
- Stay under Tell's timeout (10s by default). `tell-ntfy.sh` uses
  `curl --max-time 8`.
- Need several destinations at once? Prefer [Apprise](../apprise/README.md),
  which reports partial delivery. A script that fans out must exit nonzero if
  any destination fails.
