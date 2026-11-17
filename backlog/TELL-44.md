---
id: TELL-44
title: Notify when a wrapped command finishes with tell run
status: todo
type: feature
priority: high
labels: [notify]
created: 2026-11-17T19:51:23Z
updated: 2026-11-17T19:51:30Z
---

## Context

The most common notify use is "tell me when this finishes". Examples hand-roll it four times (examples/shell/tn.sh, examples/shell/auto-notify.zsh, examples/taskfile defer, examples/wait-for/wait-url.sh), each computing exit status, duration, and type itself. noti and ntfy (`--wait-cmd`) build it in. A built-in wrapper gives one consistent message, correct exit propagation, and a desktop click back to the origin terminal (TELL-43).

## Scope

- `tell run [flags] [--] CMD [ARG...]`: the first non-flag argument starts the command. Tell runs it in the foreground with Tell's stdin, stdout, stderr, environment, and working directory unchanged (it is the user's own command, so child-environment stripping does not apply) and no timeout, then sends one notification through the normal notify pipeline (preparation, redaction, cascade). `--timeout` covers delivery only.
- Accepts notify's routing and title flags (`--via`, provider selectors, `-t`). Type is derived: `success` on exit 0, else `failure`. No `--type` and no message argument.
- Body: the command line (display capped at about 120 characters with an ellipsis), then outcome and a humanized duration: `go test ./... finished in 1m24s`, `go test ./... failed with exit 1 after 3m32s`, `go test ./... killed by SIGKILL after 12s`. Message preparation then applies as usual.
- Exit status is the command's: its exit code, or 128+N when killed by signal N. A command that cannot start exits 127 (not found) or 126 (not executable) with one diagnostic line and sends nothing. Delivery problems print one human line on stderr and never change the exit status.
- Signals: Tell ignores SIGINT while the command runs (the terminal delivers it to the whole foreground process group, so forwarding would double it). If Tell saw SIGINT, it sends nothing (the user is at the terminal) and exits with the command's status. SIGTERM and SIGHUP sent to Tell are forwarded to the command, and the resulting exit is notified.
- `--json` is a usage error: stdout belongs to the command. Bare `tell run` is a usage error (exit 2) and runs nothing.
- Not exposed through MCP or listener actions.
- Name: `tell run` reads as "run, then tell". The listener route `run:` key is config, not a command, so there is no CLI collision.

## Non-goals

Minimum-duration thresholds, suppressing when the terminal is focused, shell hooks that notify for every command, waiting on existing PIDs or URLs (polling), output capture or tails in the message, retries, a timeout on the wrapped command, and MCP or listener exposure.

## Dependencies

None. Related: TELL-43 (click returns to origin).

## Modified-file boundaries

internal/cli (new run.go, command registration and help in cli.go), cmd/tell/main.go (signal handling, only if required), integration/ (smoke test with real signals), README.md, docs/design.md (Commands), docs/configuration.md (flags), examples/shell (replace tn.sh with `tell run`; keep auto-notify.zsh), examples/README.md.

## Verification

Build `bin/tell` and exercise exit codes, interactive I/O, Ctrl-C in a real terminal, and SIGTERM forwarding with the stderr route. Run `task ci`.

## Acceptance criteria

- [ ] tell run -- sh -c "exit 7" exits 7 and delivers one failure notification naming the command, exit 7, and the duration; tell run -- true exits 0 with one success notification; verified on the built binary with the stderr route
- [ ] A command that cannot be found exits 127 and sends nothing; bare tell run and --json are usage errors (exit 2) that run nothing
- [ ] The command keeps Tell stdin, stdout, and stderr: an interactive command reads terminal input and writes to stdout, and the notification goes only to its route; verified on the built binary
- [ ] Ctrl-C (SIGINT to the process group) stops the command, sends nothing, and exits 130; SIGTERM sent to Tell alone is forwarded and the notification reports the signal; covered by a smoke test with real signals
- [ ] A delivery failure (for example --via apprise with apprise missing) prints one diagnostic line on stderr and leaves the command exit status unchanged; covered by a test
- [ ] README.md, docs/design.md, docs/configuration.md, examples/shell, and examples/README.md document tell run; task ci passes
