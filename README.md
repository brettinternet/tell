# Tell

**Small delivery plumbing for humans and agents.**

A Go CLI for consistent notification and message routing from terminals, scripts,
and repo automation. Repo scripts own workflows; your scheduler (cron, launchd,
CI, or Noon) owns schedules, queues, and run history. Tell executes one
invocation and exits.

**Implemented:** strict layered configuration, source-aware inspection, JSON
output, non-destructive project initialization, stderr/desktop/Apprise/personal-command notifications,
message normalization, embedded secret redaction, and development tooling.
**Not implemented:** Herdr routes or agent messaging. Tell never automatically retries. These are tracked in Backlog.md; there are no successful stub commands.

## Try it

```sh
mise trust
mise exec -- task init
mise exec -- task cli:build

bin/tell                         # inspect settings; sends nothing
bin/tell --help
bin/tell --version
bin/tell init                    # create tell.yaml; never overwrite
bin/tell init --local            # create private .tell.yaml; ignore it in Git
bin/tell init --user             # create personal user config (mode 0600)
bin/tell notify "Build finished"  # default: stderr, stdout untouched
printf 'Build finished' | bin/tell notify -  # explicitly read stdin
bin/tell --json notify --type success "Build finished"
bin/tell config                  # effective values, origins, loaded files
bin/tell config check            # validate without invoking providers
bin/tell --json config check
TELL_TIMEOUT=5s bin/tell config
bin/tell --timeout 8s config     # flags override environment
bin/tell -C ../another-repo config
bin/tell --config ./ci.yaml config check  # only this file, plus env/flags
```

Configuration precedence, lowest to highest:

```text
defaults → user config → tell.yaml → .tell.yaml → TELL_* environment → CLI flags
```

User config is `$XDG_CONFIG_HOME/tell/config.yaml`, or
`~/.config/tell/config.yaml` when XDG is unset. Settings merge by field.
Shared `tell.yaml` rejects personal routing (`notify.via`, `apprise.config`,
`herdr.session`, `hcom.to`); put it in user config or private `.tell.yaml`.
Explicit `--config`, environment, and flags also accept routing.

```yaml
# tell.yaml — safe to share; provider credentials live elsewhere
version: 1
timeout: 10s
apprise:
  tag: project-alerts
```

Choose the route separately in user config:

```yaml
# ~/.config/tell/config.yaml (or $XDG_CONFIG_HOME/tell/config.yaml)
version: 1
notify:
  via: apprise
apprise:
  config: ../apprise/apprise.yaml
```

Run `tell notify "Build finished"` to deliver using the selected personal route.
The default route is stderr. Apprise is an **optional external executable** on PATH;
it owns native configuration, credentials, and provider integrations. Tell passes
explicit options only and never opens the referenced Apprise file during inspection.

Messages are one literal argument, or `-` to explicitly read stdin. Empty messages
fail before delivery. Titles and bodies reject NUL, strip terminal sequences,
controls (except newline/tab), and bidi overrides/isolates before delivery.
Titles are capped at 256 bytes and bodies at 16 KiB, on UTF-8 boundaries,
including a visible `[truncated]` marker. Before truncation, embedded Gitleaks rules
replace detected secrets in titles and bodies with `[REDACTED]`; JSON reports
`redactions`. Personal config can set `secrets.policy: reject` to refuse delivery
instead. Detection is best-effort, not a guarantee that text contains no secrets. See [preparation](docs/design.md#message-preparation).
`--title` defaults to the selected project directory name;
`--type` accepts info (default), success, warning, or failure. Provider acceptance
is **not human acknowledgment**. JSON results never contain message bodies;
timeout/cancellation after provider start means delivery is uncertain. See the
[outcome table](docs/design.md#outcomes) before deciding to resend.

See [configuration](docs/configuration.md) for all keys, flags, paths, and error
semantics; [design](docs/design.md) for the delivery boundary and planned adapters.

For a user-owned executable, set personal `notify.command: [/absolute/path/to/executable]`
and use `tell notify --via command "Build finished"`. Tell passes the prepared body
on stdin and metadata in environment variables, without shell interpolation.
See the [command contract and example](docs/design.md#command-delivery).

## Native desktop notifications

`tell notify --via desktop "Build finished"` uses macOS `osascript` or Linux
`notify-send`, without Apprise or terminal-notifier. Set `notify.via: desktop`
in personal config to make it your default; bare `tell` still only inspects.

On macOS, banners are attributed to **Script Editor**; allow its notifications
in System Settings when prompted. On Linux, install `notify-send` (libnotify)
and run within a desktop session. Scheduled services need the user's session
D-Bus, usually via `DBUS_SESSION_BUS_ADDRESS`; Tell inherits it, not discovers it.
Focus / Do Not Disturb can hide banners. Posted means accepted, not seen or
acknowledged. See [desktop behavior](docs/design.md#desktop-delivery).

## Development

```sh
task test
task ci
task backlog
task backlog:next
```

Tooling follows the sibling hum, bump (Noon), and project repositories: mise,
Task, Lefthook, Staticcheck, Gitleaks, govulncheck, git-cliff, Worktrunk, and
Backlog.md. See [development](docs/development.md).

No daemon, database, web UI, provider SDK, or scheduler is embedded.
Apprise is optional and external; Herdr/hcom adapters remain planned. None is
required for building or inspecting configuration.

MIT licensed.
