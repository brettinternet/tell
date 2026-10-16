# Configuration

Inspection (`tell`, `tell config`, `tell config check`, `tell doctor`) never
sends a message.

## Sources

Lowest to highest priority:

| # | Source | Accepts routing? |
| --- | --- | --- |
| 1 | Built-in defaults | — |
| 2 | `$XDG_CONFIG_HOME/tell/config.yaml`, else `~/.config/tell/config.yaml` | yes |
| 3 | `tell.yaml` (shared, committed) | no |
| 4 | `.tell.yaml` (private) | yes |
| 5 | `TELL_*` environment | yes |
| 6 | Flags | yes |

Settings merge by field. `notify.command` is replaced whole, never merged.
`tell config` shows each value's winning source:

```console
$ tell config
Project: "/Users/me/dev/app"
apprise.tag: "project-alerts" ("/Users/me/dev/app/tell.yaml")
notify.command: "[configured]" ("/Users/me/.config/tell/config.yaml")
notify.via: "command" ("/Users/me/.config/tell/config.yaml")
timeout: "10s" ("default")
...
Files (low → high precedence):
  "/Users/me/.config/tell/config.yaml"
  "/Users/me/dev/app/tell.yaml"
```

Missing files are skipped. Unreadable files, directories, dangling symlinks, and
malformed files are errors, even when a higher layer overrides their values.
There is no system config layer.

`--config FILE` (or `TELL_CONFIG`) replaces every automatic file. The file must
exist and is trusted as personal. Defaults, environment, and flags still apply.
HOME and XDG_CONFIG_HOME are not read, so it works for services without HOME.

### Personal routing

These fields choose where messages go, so shared `tell.yaml` rejects them:
`notify.via`, `notify.command`, `apprise.config`, `herdr.session`, `send.via`,
`send.to`, `send.from`, `secrets.policy`, and `mcp.agent_targets`.

#### MCP agent target overrides

MCP agents cannot choose routes or recipients by default. The personal-only
`mcp.agent_targets` setting defaults to `false`; set it to `true` in user
config, private `.tell.yaml`, or explicit `--config` to permit overrides for
the notify route, Herdr session, Apprise tag, send route, and one exact
recipient. This does not let agents choose `notify.command`, `apprise.config`,
`send.from`, `timeout`, or `secrets.policy`. Shared `tell.yaml` cannot grant
this authority.
The setting also accepts `TELL_MCP_AGENT_TARGETS` and
`--mcp-agent-targets`; `tell config` shows its value and winning origin. Use
`tell mcp --mcp-agent-targets=false` to revoke the opt-in for one server process.

```console
$ tell config check
tell: config /Users/me/dev/app/tell.yaml:3: notify.via is personal routing; set it in user config, .tell.yaml, --config, environment, or flags instead of shared tell.yaml
```

A higher layer cannot make them valid in `tell.yaml`. Repositories decide what
happened; personal configuration decides where it goes.

### Paths

YAML paths resolve relative to the file that declares them, before merging.
Flag and environment paths resolve relative to the current directory. Nothing
is expanded: no `~`, variables, or shell.

```yaml
# ~/.config/tell/config.yaml
apprise:
  config: ../apprise/apprise.yaml   # → ~/.config/apprise/apprise.yaml
```

`notify.command` argv is passed literally; see
[command delivery](design.md#command-delivery). `XDG_CONFIG_HOME` must be
absolute when set.

## Project discovery

`-C DIR`, `--project DIR`, or `TELL_PROJECT` selects exactly that directory.
Otherwise Tell walks up from the current directory to the nearest directory
containing `tell.yaml`, `.tell.yaml`, or `.git` (a worktree's `.git` file counts),
and never reads config above it. With no marker it uses the current directory.
Symlinks are resolved. `--config` does not change project selection. No
repository code runs during discovery.

## Settings

| YAML | Default | Environment | Flag |
| --- | --- | --- | --- |
| `timeout` | `10s` | `TELL_TIMEOUT` | `--timeout` |
| `notify.via` | `stderr` | `TELL_NOTIFY_VIA` | `--via` |
| `notify.command` | — | — | — |
| `secrets.policy` | `redact` | — | — |
| `apprise.config` | — | `TELL_APPRISE_CONFIG` | `--apprise-config` |
| `apprise.tag` | — | `TELL_APPRISE_TAG` | `--apprise-tag` |
| `apprise.format` | — | `TELL_APPRISE_FORMAT` | `--apprise-format` |
| `herdr.session` | — | `TELL_HERDR_SESSION` | `--herdr-session` |
| `send.via` | — | `TELL_SEND_VIA` | `--send-via` |
| `send.to` | — | `TELL_SEND_TO` | `--to` |
| `send.from` | `tell-automation` | `TELL_SEND_FROM` | `--from` |
| `mcp.agent_targets` | `false` | `TELL_MCP_AGENT_TARGETS` | `--mcp-agent-targets` |

A full personal config (user config or `.tell.yaml`, never `tell.yaml`):

```yaml
version: 1
timeout: 10s
notify:
  via: apprise
apprise:
  config: ../apprise/apprise.yaml
  tag: project-alerts
  format: markdown
herdr:
  session: main
send:
  via: hcom
  to: reviewer
  from: tell-automation
```

Values:

| Field | Accepts |
| --- | --- |
| `timeout` | Positive Go duration. `0s` is refused. |
| `notify.via` | `stderr`, `desktop`, `apprise`, `command`, `herdr` |
| `notify.command` | Non-empty argv list; non-blank executable; no NUL. File-only (user, `.tell.yaml`, `--config`). |
| `secrets.policy` | `redact` (replace with `[REDACTED]`) or `reject` (exit 1, `secret_detected`). File-only. |
| `apprise.config` | Local file path, not a URL. Tell never opens it. |
| `apprise.tag` | One Apprise tag expression, passed through. |
| `apprise.format` | `text`, `markdown`, `html` |
| `herdr.session` | Existing session name. Use `default` to mean the default session. |
| `send.via` | `hcom` or `herdr`. No default. |
| `send.to` | One exact hcom full name (no `@`) or Herdr agent name/pane ID. |
| `send.from` | ≤64 ASCII letters, digits, `_`, `-`; not starting with `-`. |
| `mcp.agent_targets` | `true` or `false`; personal-only; defaults to `false` |

`send.to` refuses whitespace, lists, wildcards, tag selectors ending in `-`,
remote hcom targets, and `self`. A missing or invalid recipient is `not_sent`;
there is no broadcast fallback.

Optional settings are unset until supplied. An explicitly empty value is an
error (except `send.to`); omit the key instead. A flag replaces an environment
value before validation. Provider URLs and credentials are not accepted
anywhere; keep them with the provider.

Removed bootstrap settings are rejected: `notify.via: stdout`, `hcom.to`, and
`TELL_HCOM_TO` (use `send.to` and `TELL_SEND_TO`).

## Invocation flags

Message flags are per invocation, not settings:

```sh
tell notify --title "CI" --type failure "Tests failed"   # --type: info|success|warning|failure
tell notify - < message.txt                              # stdin only with -
tell send --intent request "Triage PRs"                  # --intent: inform|request
tell --send-via herdr --to reviewer send "..."           # --via is notify-only
```

`--title` defaults to the project directory name. Empty messages are refused.
Text is normalized and scanned before delivery; see
[message preparation](design.md#message-preparation). Scanning is best-effort,
and desktop, Herdr, and Apprise receive the body in argv, which local process
inspection can see. Do not put secrets in messages.

## Route readiness

`tell doctor` checks routes without sending, starting services, or repairing
anything. Run it as the same user, with the same environment, as the real caller.

```console
$ tell doctor
desktop (selected): ready
apprise: missing_executable

$ tell --json doctor
{"schema_version":1,"selected":"stderr","ready":true,"routes":[{"route":"stderr","ready":true,"reason":"ready"}]}
```

It checks the selected route, then every explicitly configured one (any Apprise
setting, `herdr.session`, `notify.command`), plus `send/hcom` or `send/herdr`
when `send.via`/`send.to` are set. Defaults alone add nothing; use
`tell --via desktop doctor` to check desktop.

| Route | Check | Does not verify |
| --- | --- | --- |
| stderr | nothing needed | — |
| desktop | `osascript` (macOS) or `notify-send` (Linux) on PATH | permissions, D-Bus, Focus, visible banners |
| command | executable resolves and is executable; never run | interpreter, the command's own config |
| apprise | `apprise --dry-run` with effective config/tag/format; ≥1 matching service | credentials, reachability |
| herdr | `herdr session list --json` has the exact session running | — |
| send/hcom | `hcom list --json` has one exact active name | — |
| send/herdr | `herdr agent get` reports idle or done | state at send time |

Zero Apprise matches or a failed dry run is not ready. Working/unknown agents
are busy; dialogs are blocked. Each probe uses the configured timeout. Output
is capped at 1 MiB; oversized or malformed output fails closed. No URLs, raw
config, or provider diagnostics are printed. Inherited `HERDR_*` variables are
removed.

JSON adds `selected_send` when agent messaging is configured, and `services`
for Apprise when collected. Exit 0 means the selected notify route and any
configured send route are ready, even if another route is not. Exit 1 means one
is not ready, or configuration/output failed. Exit 2 is a usage error.

## Strict YAML

One document, one mapping. Rejected: unknown or duplicate keys, aliases,
anchors, merge keys, nulls, wrong types, unsupported versions, files over
1 MiB. Empty or comment-only files count as an empty layer. `version` is
optional and defaults to 1. Use nested mappings, not dotted keys.

Errors name the source; semantic errors also name the field and line. Errors
never echo raw values.

## Inspection output

`tell`, `tell config`, and `tell config show` print effective settings and
origins. `tell config check` validates the same snapshot. Neither runs
providers. Human output quotes values so terminal escapes are inert.
`notify.command` shows only `[configured]`; JSON omits it but keeps its origin.

```console
$ tell --json config check
{"config":{"project":"/Users/me/dev/app","timeout":"10s","notify":{"via":"stderr"},...,"origins":{...}},"schema_version":1,"valid":true}

$ tell --json --timeout 0s config check; echo $?
{"error":{"code":"config","message":"flag:--timeout: timeout must be a positive Go duration, such as 10s"},"schema_version":1}
1
```

`--json` prints one stdout object with `schema_version: 1`. Human errors go to
stderr. Exit 0 success, 1 configuration/I/O failure, 2 usage. Help, version,
and completion stay plain text. Delivery uses its own
[outcomes](design.md#outcomes). In notify JSON, `route` is empty when parsing
or configuration failed before a route was resolved.

## Initialization

| Command | Creates | Mode | Contents |
| --- | --- | --- | --- |
| `tell init` | `tell.yaml` | default | commented shared fields |
| `tell init --local` | `.tell.yaml` | 0600 | commented personal routing |
| `tell init --user` | user config (creates parent dirs) | 0600 | commented personal routing |

```console
$ tell init
Created "/Users/me/dev/app/tell.yaml"
Ignore .tell.yaml in Git. Next: tell config check

$ tell init
Kept existing "/Users/me/dev/app/tell.yaml" (not overwritten)
Ignore .tell.yaml in Git. Next: tell config check
```

Existing files are kept byte-for-byte, even malformed ones. Existing symlinks
and directories are refused. Init does not edit `.gitignore`, copy inherited
values, or read provider config. It refuses `--config` and setting overrides
instead of ignoring them. `--user` cannot combine with `--local` or `--project`.
