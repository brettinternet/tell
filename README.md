# Tell

**Small delivery plumbing for humans and agents.**

A Go CLI for consistent notification and message routing from terminals, scripts,
and repo automation. Repo scripts own workflows; Noon owns schedules, queues, and
run history. Tell executes one invocation and exits.

**Implemented:** strict layered configuration, source-aware inspection, JSON
output, non-destructive project initialization, stderr/Apprise notifications, and development tooling.
**Not implemented:** desktop/Herdr/command routes, agent messaging, message
normalization or secret redaction. Tell never automatically retries. These are tracked in Backlog.md; there are no successful stub commands.

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
fail before delivery. `--title` defaults to the selected project directory name;
`--type` accepts info (default), success, warning, or failure. Provider acceptance
is **not human acknowledgment**. JSON results never contain message bodies;
timeout/cancellation after provider start means delivery is uncertain. See the
[outcome table](docs/design.md#outcomes) before deciding to resend.

See [configuration](docs/configuration.md) for all keys, flags, paths, and error
semantics; [design](docs/design.md) for the delivery boundary and planned adapters.

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
