# Shell: tell me when it's done

You start a slow command, switch to the browser, and forget about it. These
helpers notify when it finishes.

Pick a route you will notice in personal config, for example
`notify.via: desktop` in `~/.config/tell/config.yaml`. The default stderr
route only prints to the same terminal.

## `tn`: wrap one command

[tn.sh](tn.sh) defines a `tn` function for bash and zsh:

```sh
. ~/dev/tell/examples/shell/tn.sh

tn go test ./...
tn docker build -t app .
tn ollama pull qwen3:32b
tn bun run build && tn bun run e2e
```

```text
Done: go               go test ./... finished in 84s
Failed: docker         docker build -t app . exited 1 after 212s
```

`tn` returns the command's exit status, so it composes with `&&` and `||`.

## Automatic: notify for anything slow (zsh)

[auto-notify.zsh](auto-notify.zsh) uses zsh `preexec`/`precmd` hooks to notify
whenever a command runs 30 seconds or longer. Editors, pagers, SSH, and agent
TUIs are skipped.

```sh
# ~/.zshrc
export TELL_LONG_SECONDS=45
. ~/dev/tell/examples/shell/auto-notify.zsh
```

Delivery runs in the background (`&!`) with a 5-second timeout so the prompt
never waits on it. Output is discarded, so use a route other than stderr.

## Notes

- The command line goes into the notification. Tell redacts detected secrets,
  but prefer passing credentials through the environment rather than argv.
- One-off: `make release; tell notify "release: exit $?"`.
