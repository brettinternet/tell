# Tell

Send one notification to a person, or one message to an agent, then exit.

```sh
tell notify "Build finished"                         # person: stderr, desktop, Apprise, Herdr, or your command
tell send --intent request "Triage PRs; report blockers"   # agent: hcom mailbox or idle Herdr pane
```

Tell is a Go CLI for terminals, scripts, and scheduler hooks. Your scheduler
(cron, launchd, CI, Noon) owns schedules, retries, and history. Your scripts
decide what is worth saying. Tell routes one message and reports what happened.
It has no daemon, queue, database, or retries.

## Installation

Install with Homebrew:

```sh
brew trust --formula brettinternet/tap/tell
brew install brettinternet/tap/tell
```

Or use mise:

```sh
mise exec github:brettinternet/tell@latest -- tell --version
# Or install persistently:
mise use -g github:brettinternet/tell@latest
```

No asset selection override or custom plugin is needed. To install manually,
download `tell-vX.Y.Z-OS-ARCH.tar.gz` and `checksums.txt` from
[GitHub Releases](https://github.com/brettinternet/tell/releases).
OS is `macos` or `linux`; ARCH is `arm64` or `x64`. For example,
after downloading the macOS Apple Silicon archive (replace the version):

```sh
grep '  tell-vX.Y.Z-macos-arm64.tar.gz$' checksums.txt | shasum -a 256 -c -
tar -xzf tell-vX.Y.Z-macos-arm64.tar.gz
install -m 755 bin/tell ~/.local/bin/tell  # create ~/.local/bin first; keep it on PATH
tell --version
```

Archives contain `bin/tell`. Checksums detect corruption, not publisher identity.
Homebrew and mise support macOS and Linux on arm64 and x64. Windows is not supported.

Tell itself needs no provider runtime for inspection or stderr delivery.
Optional routes require their external executable on PATH: Apprise with its
own provider configuration; Herdr with an existing explicit session; hcom with
a participating recipient; desktop uses macOS osascript or Linux notify-send
and a desktop D-Bus session. The command route requires your own executable.
Credentials remain with each provider. Run `tell doctor` before delivery.

`tell --version` reports the version (without the tag's `v`), source commit,
and build timestamp. Source-only `task cli:build` remains `dev`; versioned
archives use [the packaging task](docs/development.md#release-packaging).
See the [compatibility promise](docs/design.md#compatibility).

## Build

```sh
mise trust
mise exec -- task init
mise exec -- task cli:build   # → bin/tell
```

Apprise, Herdr, and hcom are optional external tools. None is needed to build
or inspect configuration.

## Quick tour

Bare `tell` only inspects. It never sends.

```console
$ tell
Project: "/Users/me/dev/app"
notify.via: "stderr" ("default")
secrets.policy: "redact" ("default")
timeout: "10s" ("default")
...
Inspection only; notify delivers via the selected route. Provider acceptance is not human acknowledgment.
```

The default route writes to stderr, leaving stdout for your script:

```console
$ tell notify "Build finished"
Build finished

$ tell --json notify --type success "Build finished"
Build finished
{"outcome":"accepted","redactions":0,"route":"stderr","schema_version":1}

$ printf 'Build finished' | tell notify -     # stdin only when you pass -
```

Detected secrets are redacted before delivery:

```console
$ tell --json notify "key $AWS_ACCESS_KEY_ID done"
key [REDACTED] done
{"outcome":"accepted","redactions":1,"route":"stderr","schema_version":1}
```

Check routes without sending anything. Only the selected route decides the exit code:

```console
$ tell doctor
desktop (selected): ready
apprise: not ready: apprise was not found on PATH; install it or choose another route
```

## Configure

Create starter files. `init` never overwrites.

```sh
tell init           # tell.yaml: shared, committed
tell init --user    # ~/.config/tell/config.yaml: personal (mode 0600)
```

Shared `tell.yaml` holds shared settings only. Destinations are personal, so
Tell rejects routing fields there. Personal settings belong in user config or
an explicit `--config`/`TELL_CONFIG` file. Tell does not automatically load
`.tell.yaml`, even if a repository commits one:

```yaml
# tell.yaml
version: 1
apprise:
  tag: project-alerts
```

```yaml
# ~/.config/tell/config.yaml
version: 1
notify:
  via: desktop
```

```console
$ tell config check   # with notify.via in tell.yaml
tell: config /Users/me/dev/app/tell.yaml:3: notify.via is personal routing; set it in user config, --config, environment, or flags instead of shared tell.yaml
```

Precedence, lowest to highest:

```text
defaults → user config → tell.yaml → TELL_* env → flags
```

```sh
TELL_TIMEOUT=5s tell config       # environment
tell --timeout 8s config          # flag beats environment
tell -C ../other-repo config      # pick the project
tell --config ./ci.yaml config    # use only this file (plus env and flags)
```

Invalid settings name their source and the fix, without echoing the value:

```console
$ tell --timeout 5 notify "Build finished"
tell: --timeout: timeout must be a positive Go duration, such as 10s
```

See [configuration](docs/configuration.md) for every key.

## Notification routes

| Route | Example | Needs |
| --- | --- | --- |
| stderr (default) | `tell notify "Done"` | nothing |
| desktop | `tell notify --via desktop "Done"` | `osascript` (macOS) or `notify-send` (Linux) |
| apprise | `tell notify --via apprise "Done"` | `apprise` on PATH and its own config |
| herdr | `tell notify --via herdr --herdr-session work "Done"` | a running Herdr session |
| command | `tell notify --via command "Done"` | personal `notify.command` |

`--title` defaults to the project directory name. `--type` is `info` (default),
`success`, `warning`, or `failure`.

The command route runs your executable with the body on stdin:

```yaml
# ~/.config/tell/config.yaml
notify:
  via: command
  command: [/Users/me/.local/bin/tell-speak]
```

```sh
#!/bin/sh
# /Users/me/.local/bin/tell-speak; also gets TELL_TITLE, TELL_TYPE, TELL_PROJECT
exec /usr/bin/say -f -
```

Route details: [desktop](docs/design.md#desktop-delivery),
[Herdr](docs/design.md#herdr-delivery), [command](docs/design.md#command-delivery).

## Agent messages

Pick one recipient in personal config:

```yaml
send:
  via: hcom        # or herdr (also needs herdr.session)
  to: reviewer
  from: tell-automation
```

```console
$ tell send "Tests failed; see noon logs 42"
tell send: route="hcom" recipient="reviewer" intent="inform" outcome=accepted

$ tell send --intent request "Triage new PRs and report blockers"
```

hcom queues the message for the agent's next hook without interrupting it.
Herdr types a new turn only when the agent is idle, but the agent can start
working between the check and the prompt. Use hcom when interruption matters.
Human failures add an explanation after the receipt, including the setting, flag,
and environment variable for missing routing configuration. JSON keeps its existing
reason codes and messages.
Accepted means dispatched, not done. See [agent delivery](docs/design.md#agent-delivery).

## MCP server

`tell mcp` serves three tools—`notify`, `send`, and read-only `doctor`—over
stdio for a client-owned process. Add it to Claude Code or Codex:

```sh
claude mcp add tell -- tell mcp
codex mcp add tell -- tell mcp
```

For another client, configure the command and arguments as `tell` and `mcp`.
Protocol messages use stdout; the stderr notification route remains on stderr.
A tool call performs at most one delivery and returns a body-free result.

MCP agents cannot select routes or recipients by default. To let agents select
among your already configured routes and one exact recipient, enable the
personal-only setting `mcp.agent_targets: true` in user config, an explicit
`--config` file, environment, or flags.
This delegates destination choice, not executable paths, provider config,
sender identity, timeouts, or secret policy. Do not enable it in shared
`tell.yaml`.

```yaml
# ~/.config/tell/config.yaml
mcp:
  agent_targets: true
```

Use `tell mcp --mcp-agent-targets=false` to disable overrides for one client
invocation. See [MCP trust and result behavior](docs/design.md#mcp) and the
[setting reference](docs/configuration.md#mcp-agent-target-overrides).

## Outcomes

Each `notify` or `send` exits with one outcome. It tells you whether resending is safe.

| Exit | Outcome | Resend? |
| --- | --- | --- |
| 0 | accepted | No. Accepted is not read or acknowledged. |
| 1 | config | After fixing. Nothing was sent. |
| 2 | usage | After fixing. Nothing was sent. |
| 3 | not_sent | Yes, or use another route. |
| 4 | failed | Not blindly. Some destinations may have it. |
| 5 | uncertain | Not without checking. It may have arrived. |

See [outcomes](docs/design.md#outcomes) and [message preparation](docs/design.md#message-preparation).

## Examples

[examples/](examples/README.md) has copyable recipes: notify when slow
commands finish, Claude Code and Codex hooks, MCP agents, CI and test
watchers that hand failures to agents, transition-only scheduled checks,
phone push via ntfy or Apprise, and safe fallback chains.
[examples/noon](examples/noon/README.md) has a Noon hook that notifies only on
failure and recovery, plus a [scheduled agent instruction](examples/noon/README.md#scheduled-instructions).

## Docs

- [Configuration](docs/configuration.md): files, keys, readiness, init
- [Design](docs/design.md): boundaries, outcomes, each route's exact behavior
- [Development](docs/development.md): tasks, layout, testing

MIT licensed.
