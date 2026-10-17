# Test watcher → agent: "you just broke the tests"

A coding agent edits files faster than it runs the suite. A watcher in another
pane runs tests on every change; [report-tests.sh](report-tests.sh) tells the
agent only when the result flips.

```sh
watchexec --exts go,mod --debounce 2s -- examples/test-watch-agent/report-tests.sh go test ./...
watchexec --exts ts,tsx -- examples/test-watch-agent/report-tests.sh bun test
```

The agent receives, at its next hook:

```text
[Tell automation: test-watch; route: hcom; intent: inform]
Tests started failing after recent edits in /Users/me/dev/api. Reproduce: go test ./...
```

and later, `Tests pass again in /Users/me/dev/api (go test ./...).`

## Setup

```yaml
# ~/.config/tell/config.yaml
version: 1
send:
  via: hcom
  to: impl
  from: test-watch
```

Or per pane: `TELL_SEND_TO=impl watchexec ...` when several agents work in
different checkouts.

## Design choices

- **inform, not request.** The agent decides what to do; there is no reply
  obligation, and its current turn is not steered.
- **hcom, not Herdr.** hcom queues for the next hook. Herdr would type into
  the agent's prompt when idle, which is noisier for a status update.
- **Transitions only.** Repeating "still failing" every save is noise. The
  state file lives in `$TMPDIR`, one per checkout.
- **Pointers only.** The message says how to reproduce, not what failed.
  The test output stays in your pane.
