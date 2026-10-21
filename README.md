# Tell

[![CI](https://github.com/brettinternet/tell/actions/workflows/ci.yaml/badge.svg)](https://github.com/brettinternet/tell/actions/workflows/ci.yaml)

Send one notification to a person, or one message to an agent, then exit.

```sh
tell notify "Build finished"                               # person: stderr, desktop, Apprise, Herdr, or your command
tell send --intent request "Triage PRs; report blockers"   # agent: hcom mailbox or idle Herdr pane
tell listen slack github                                   # foreground: route Slack/GitHub events to actions
```

| Who | Owns |
| --- | --- |
| Your scheduler (cron, launchd, CI, Noon) | schedules, retries, history |
| Your scripts | what is worth saying |
| Tell | routing one message and reporting what happened |

Tell has no daemon, durable queue, database, or same-route retries.

## Installation

```sh
brew trust --formula brettinternet/tap/tell
brew install brettinternet/tap/tell
# or
mise use -g github:brettinternet/tell@latest
```

Or download `tell-vX.Y.Z-{macos,linux}-{arm64,x64}.tar.gz` and `checksums.txt`
from [Releases](https://github.com/brettinternet/tell/releases):

```sh
grep '  tell-vX.Y.Z-macos-arm64.tar.gz$' checksums.txt | shasum -a 256 -c -
tar -xzf tell-vX.Y.Z-macos-arm64.tar.gz
install -m 755 bin/tell ~/.local/bin/tell
tell --version   # tell 0.18.2 (47b389f, built 2026-10-16T21:43:17Z)
```

Checksums detect corruption, not publisher identity. Windows is not supported.

Optional routes need their own executable on PATH:

| Route | Needs |
| --- | --- |
| stderr | nothing |
| desktop | `osascript` (macOS) or `notify-send` + desktop D-Bus (Linux) |
| apprise | `apprise` and its own config |
| herdr | `herdr` and a running session |
| hcom | `hcom` and a participating recipient |
| command, listener `run` | your executable |

See the [compatibility promise](docs/design.md#compatibility).

## Build

```sh
mise trust
task init
task cli:build   # → bin/tell (version "dev"; see docs/development.md#release-packaging)
```

## Quick tour

Bare `tell` only inspects; it never sends or runs providers.

```console
$ tell
Project: /Users/me/dev/app
Config files:
  none
Notify: stderr (default)
Send: not configured

Next: tell notify "hello" · tell doctor · tell init · tell --help
```

The default route writes to stderr, leaving stdout for your script:

```console
$ tell notify Build finished                  # arguments join with single spaces
Build finished

$ tell --json notify --type success "Build finished"
Build finished
{"outcome":"accepted","redactions":0,"route":"stderr","schema_version":1}

$ printf 'Build finished' | tell notify -     # stdin only for a lone -
```

Secrets are redacted before delivery:

```console
$ tell --json notify "key $AWS_ACCESS_KEY_ID done"
key [REDACTED] done
{"outcome":"accepted","redactions":1,"route":"stderr","schema_version":1}
```

Check routes without sending:

```console
$ tell doctor
desktop (selected): ready
apprise: not ready
  apprise was not found on PATH; install it or choose another route
```

## Configure

```sh
tell init           # tell.yaml: shared, committed
tell init --user    # ~/.config/tell/config.yaml: personal (0600)
tell init --local   # .git/tell.yaml: personal, this repo and its worktrees (0600)
```

Repositories decide what happened; personal config decides where it goes.
Shared `tell.yaml` rejects routing:

```yaml
# tell.yaml
apprise:
  tag: project-alerts
```

```yaml
# ~/.config/tell/config.yaml
notify:
  via: desktop
```

```console
$ tell config check   # with notify.via in tell.yaml
tell: config /Users/me/dev/app/tell.yaml:3: notify.via is personal routing; set it in user config, .git/tell.yaml, --config, environment, or flags instead of shared tell.yaml
```

```text
defaults → user config → tell.yaml → .git/tell.yaml → TELL_* env → flags
```

```sh
TELL_TIMEOUT=5s tell config       # environment
tell --timeout 8s config          # flag beats environment
tell -C ../other-repo config      # pick the project
tell --config ./ci.yaml config    # only this file (plus env and flags)
tell doctor --via desktop         # routing flags go after the command
```

See [configuration](docs/configuration.md) for every key and flag, and
[editor schemas](docs/configuration.md#editor-schemas).

## Notification routes

| Route | Example |
| --- | --- |
| stderr (default) | `tell notify "Done"` |
| desktop | `tell notify --via desktop "Done"` |
| apprise | `tell notify --via apprise --apprise-tag phone "Done"` |
| herdr | `tell notify --via herdr --herdr-session work "Done"` |
| command | `tell notify --via command "Done"` |

`-t`/`--title` defaults to the project directory name; `-n`/`--type` is
`info`, `success`, `warning`, or `failure`.

The command route runs your executable with the body on stdin:

```yaml
notify:
  via: command
  command: [/Users/me/.local/bin/tell-speak]
```

```sh
#!/bin/sh
# also gets TELL_TITLE, TELL_TYPE, TELL_PROJECT
exec /usr/bin/say -f -
```

### Ordered notify fallback

A list tries routes in order:

```yaml
notify:
  via:
    - apprise: {tag: phone}
    - apprise: {urls_env: TELL_CHAT_APPRISE_URLS}   # URLs stay in the environment
    - desktop
  fallback_notice: title    # later titles name the failed first step
```

| Step outcome | Cascade |
| --- | --- |
| accepted | stops |
| not_sent | advances |
| failed | stops unless `fallback_on: [not_sent, failed]` |
| uncertain | always stops; it may have arrived |

One `--timeout` covers the whole cascade. `--via` or `TELL_NOTIFY_VIA` picks a
single route. `send` has no fallback: two agents doing the same work is worse
than none. Details: [fallback](docs/design.md#notify-fallback-cascades),
[examples](examples/fallback/README.md), [desktop](docs/design.md#desktop-delivery),
[Herdr](docs/design.md#herdr-delivery), [command](docs/design.md#command-delivery).

## Agent messages

```yaml
send:
  via: hcom        # or herdr (also needs herdr.session)
  to: reviewer
```

```console
$ tell send "Tests failed; see noon logs 42"
tell send: route="hcom" recipient="reviewer" intent="inform" outcome=accepted

$ tell send hi        # unconfigured
tell send: route="" recipient="" intent="inform" outcome=not_sent reason=missing_route
tell send: choose send.via in personal config, --send-via, or TELL_SEND_VIA (hcom or herdr)
```

hcom queues for the agent's next hook. Herdr types a turn only into an idle
agent, but the agent can start working between check and prompt; use hcom when
interruption matters. Accepted means dispatched, not done.
See [agent delivery](docs/design.md#agent-delivery).

## MCP server

`tell mcp` serves `notify`, `send`, and read-only `doctor` over stdio:

```sh
claude mcp add tell -- tell mcp
codex mcp add tell -- tell mcp
```

Agents use your configured routes and cannot pick destinations. To let them
choose among configured routes and one exact recipient, opt in personally:

```yaml
# ~/.config/tell/config.yaml (never tell.yaml)
mcp:
  agent_targets: true
```

`tell mcp --mcp-agent-targets=false` revokes it for one client. Agents never
choose command executables, `urls_env`, credentials, sender, timeout, or
secret policy. See [MCP](docs/design.md#mcp) and
[the setting](docs/configuration.md#mcp-agent-target-overrides).

## Slack event capture

1. Enable Socket Mode on a Slack app and create an app token with `connections:write`.
2. Subscribe to bot events and scopes (e.g. `channel_created` + `channels:read`), then install the app.
3. Export the token as `TELL_SLACK_APP_TOKEN` and name it in personal config:

```yaml
# ~/.config/tell/config.yaml
listen:
  slack:
    app_token_env: TELL_SLACK_APP_TOKEN     # the variable name, never the token
  routes:
    - name: incident-kickoff
      source: slack
      type: channel_created
      match:
        event.channel.name: {prefix: "inc-"}
      run: [/Users/me/.local/bin/incident-to-noon]   # envelope JSON on stdin
    - name: notify-channel
      source: slack
      type: channel_created
      notify:
        title: "Channel ${data.event.channel.name} created"
        message: "Slack event ${id}"
```

```sh
tell listen                    # inspect only; no connections
tell listen --dry-run slack    # match and validate; deliver nothing
tell listen slack              # run matching actions
tell listen --print slack      # sensitive NDJSON export; no actions
```

Each route has one `run`, `notify`, or inform-only `send` action. Run one
listener per Slack app (Slack splits deliveries across connections). Events
are held only in memory; restart or sleep can lose them. See
[listener examples](examples/listen/README.md) and the
[full contract](docs/listening.md).

## GitHub repository webhooks

```yaml
# ~/.config/tell/config.yaml
listen:
  github:
    secret_env: TELL_GITHUB_WEBHOOK_SECRET
    repository_ids: [123456789]      # numeric repository.id allowlist
  http:
    addr: 127.0.0.1:8787             # loopback only
  routes:
    - name: opened-pr
      source: github
      type: pull_request
      match: {action: opened}
      run: [/Users/me/.local/bin/handle-pr-event]
```

Run a named Cloudflare Tunnel yourself, forwarding only `/github`:

```yaml
# cloudflared config.yaml
tunnel: <TUNNEL-UUID>
credentials-file: /Users/me/.cloudflared/<TUNNEL-UUID>.json
ingress:
  - hostname: hooks.example.com
    path: ^/github$
    service: http://127.0.0.1:8787
  - service: http_status:404
```

In the repository's **Settings → Webhooks**, add
`https://hooks.example.com/github` with `application/json`, TLS verification,
and the same secret. Then run `tell listen github`.

Tell verifies the HMAC before parsing; the tunnel is transport, not
authorization. A 202 means received in memory, not that an action succeeded.
GitHub does not redeliver failures automatically, so reconcile important state.
Organization hooks and GitHub Apps are unsupported.
See [GitHub webhooks](docs/listening.md#github-webhooks).

## Outcomes

| Exit | Outcome | Resend? |
| --- | --- | --- |
| 0 | accepted | No. Not proof anyone read it. |
| 1 | config | After fixing. Nothing sent. |
| 2 | usage | After fixing. Nothing sent. |
| 3 | not_sent | Yes, or use another route. |
| 4 | failed | Not blindly. Some destinations may have it. |
| 5 | uncertain | Not without checking. It may have arrived. |

See [outcomes](docs/design.md#outcomes) and [message preparation](docs/design.md#message-preparation).

## Terminal output and completion

Styling is per stream and disabled for pipes, JSON, MCP, `--no-color`,
`NO_COLOR`, and `TERM=dumb`.

```bash
source <(tell completion bash)                          # ~/.bashrc
```

```zsh
autoload -Uz compinit && compinit                       # ~/.zshrc
source <(tell completion zsh)
```

```fish
mkdir -p ~/.config/fish/completions
tell completion fish > ~/.config/fish/completions/tell.fish   # regenerate after upgrades
```

## Examples

[examples/](examples/README.md): slow-command notifications, Claude Code and
Codex hooks, MCP agents, CI watchers that hand failures to agents,
transition-only checks, Slack routes and a [Noon handoff](examples/noon/README.md),
phone push via ntfy or Apprise, and fallback chains.

## Docs

- [Configuration](docs/configuration.md): files, keys, readiness, init
- [Design](docs/design.md): boundaries, outcomes, route behavior
- [Listening](docs/listening.md): Slack and GitHub listener contract
- [Development](docs/development.md): tasks, layout, testing, releases
- [Changelog](CHANGELOG.md)

MIT licensed.
