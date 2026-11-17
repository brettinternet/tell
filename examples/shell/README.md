# Shell: tell me when it's done

You start a slow command, switch to the browser, and forget about it. These
helpers notify when it finishes.

Pick a route you will notice in personal config, for example
`notify.via: desktop` in `~/.config/tell/config.yaml`. The default stderr
route only prints to the same terminal.

## `tell run`: wrap one command

```sh
tell run go test ./...
tell run docker build -t app .
tell run ollama pull qwen3:32b
tell run bun run build && tell run bun run e2e
alias tn='tell run'   # optional shorthand
```

```text
go test ./... finished in 1m24s
docker build -t app . failed with exit 1 after 3m32s
```

`tell run` exits with the command's status, so it composes with `&&` and `||`.
Flags after the command belong to it; put Tell's flags first
(`tell run --via desktop -- make deploy`). Ctrl-C stops the command without a
notification.

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
