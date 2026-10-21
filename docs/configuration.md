# Configuration

Inspection (`tell`, `tell config`, `tell config check`, `tell doctor`) never
sends a message.

## Sources

Lowest to highest priority:

| # | Source | Routing allowed? |
| --- | --- | --- |
| 1 | built-in defaults | — |
| 2 | `$XDG_CONFIG_HOME/tell/config.yaml`, else `~/.config/tell/config.yaml` | yes |
| 3 | `tell.yaml` (shared, committed) | no |
| 4 | `.git/tell.yaml` (personal, this repository) | yes |
| 5 | `TELL_*` environment | yes |
| 6 | flags | yes |

Settings merge by field. `notify.command`, list-valued `notify.via`, and
`listen.routes` replace lower values whole.

`.git/tell.yaml` lives in the common Git directory, so linked worktrees share
it and clones cannot supply it. Tell resolves `.git` and `commondir` pointer
files itself; it never runs `git` or reads `GIT_DIR`.

`-c FILE` / `--config FILE` / `TELL_CONFIG` replaces every automatic file. The
file must exist and is trusted as personal. Defaults, environment, and flags
still apply; HOME is not needed.

Missing files are skipped. Unreadable, dangling, directory, or malformed files
are errors even when a higher layer overrides them. There is no system layer.

```console
$ tell config
Project: /Users/me/dev/app
Files (low → high precedence):
  /Users/me/.config/tell/config.yaml
  /Users/me/dev/app/tell.yaml

KEY                VALUE           ORIGIN
apprise.tag        project-alerts  /Users/me/dev/app/tell.yaml
notify.command     [configured]    /Users/me/.config/tell/config.yaml
notify.via         command         /Users/me/.config/tell/config.yaml
timeout            10s             default
...
```

Origins are `default`, a file path, `env NAME`, or `flag --name`.

### Personal routing

These fields choose destinations, so shared `tell.yaml` rejects them even if a
higher layer overrides them: `notify.via`, `notify.fallback_on`,
`notify.fallback_notice`, `notify.command`, `apprise.config`, `herdr.session`,
`send.via`, `send.to`, `send.from`, `secrets.policy`, `mcp.agent_targets`, and
the whole `listen` block.

```console
$ tell config check
tell: config /Users/me/dev/app/tell.yaml:3: notify.via is personal routing; set it in user config, .git/tell.yaml, --config, environment, or flags instead of shared tell.yaml
```

#### MCP agent target overrides

`mcp.agent_targets` (default `false`; `TELL_MCP_AGENT_TARGETS`,
`--mcp-agent-targets`) lets MCP agents choose:

| Agents may choose | Agents never choose |
| --- | --- |
| notify route (one route, disables the cascade) | `notify.command` |
| Herdr session, Apprise config/tag/format (applied to matching steps) | `send.from` |
| send route and one exact recipient | `timeout`, `secrets.policy` |

`tell mcp --mcp-agent-targets=false` revokes it for one server process.

### Paths

```yaml
# ~/.config/tell/config.yaml
apprise:
  config: ../apprise/apprise.yaml   # → ~/.config/apprise/apprise.yaml
```

YAML paths, including cascade step `config`, resolve relative to their file.
Flag and environment paths resolve relative to the current directory. Nothing
is expanded (`~`, variables, shell). `notify.command` argv is literal; see
[command delivery](design.md#command-delivery). `XDG_CONFIG_HOME` must be
absolute.

### Notify cascade

```yaml
notify:
  via:
    - apprise: {tag: phone}
    - apprise: {tag: chat}
    - desktop
  fallback_on: [not_sent]    # default; add failed to opt in
  fallback_notice: title
```

A higher-layer string `via` replaces a lower cascade with its `fallback_on`
and `fallback_notice`. `--via`, `TELL_NOTIFY_VIA`, and MCP `via` select one
route. See [fallback cascades](design.md#notify-fallback-cascades).

### Inbound listeners

The personal `listen` block configures Slack, GitHub, the loopback HTTP
address, and routes. Keys, validation, and examples are in
[listening](listening.md#configuration); setup is in the README for
[Slack](../README.md#slack-event-capture) and
[GitHub](../README.md#github-repository-webhooks).

## Project discovery

`-C DIR`, `--project DIR`, or `TELL_PROJECT` selects exactly that directory.
Otherwise Tell walks up to the nearest directory containing `tell.yaml` or
`.git`, else uses the current directory. Symlinks are resolved; `--config`
does not change the project; no repository code runs.

## Settings

| YAML | Default | Environment | Flag |
| --- | --- | --- | --- |
| `timeout` | `10s` | `TELL_TIMEOUT` | `--timeout` |
| `notify.via` | `stderr` | `TELL_NOTIFY_VIA` | `--via` |
| `notify.fallback_on` | `[not_sent]` | — | — |
| `notify.fallback_notice` | `none` | — | — |
| `notify.command` | — | — | — |
| `secrets.policy` | `redact` | — | — |
| `apprise.config` | — | `TELL_APPRISE_CONFIG` | `--apprise-config` |
| `apprise.tag` | — | `TELL_APPRISE_TAG` | `--apprise-tag` |
| `apprise.format` | — | `TELL_APPRISE_FORMAT` | `--apprise-format` |
| `herdr.session` | — | `TELL_HERDR_SESSION` | `--herdr-session` |
| `send.via` | — | `TELL_SEND_VIA` | `--send-via` |
| `send.to` | — | `TELL_SEND_TO` | `--to` |
| `send.from` | `tell-automation` | `TELL_SEND_FROM` | `--from` |
| `listen.slack.app_token_env` | — | — | — |
| `listen.github.secret_env` | — | — | — |
| `listen.github.repository_ids` | — | — | — |
| `listen.http.addr` | `127.0.0.1:8787` | — | — |
| `listen.routes` | — | — | — |
| `mcp.agent_targets` | `false` | `TELL_MCP_AGENT_TARGETS` | `--mcp-agent-targets` |

```yaml
# personal config (never tell.yaml)
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

| Field | Accepts |
| --- | --- |
| `timeout` | positive Go duration (`0s` refused) |
| `notify.via` | `stderr`, `desktop`, `apprise`, `command`, `herdr`, or a nonempty list of those names, `apprise: {config, tag, format, urls_env}`, and `herdr: {session}` steps |
| `notify.fallback_on` | nonempty subset of `not_sent`, `failed`; cascade only |
| `notify.fallback_notice` | `none` or `title` (prefix later titles with the first step's label and reason); cascade only |
| `notify.command` | nonempty argv list; nonblank executable; no NUL |
| `secrets.policy` | `redact` (`[REDACTED]`) or `reject` (exit 1, `secret_detected`) |
| `apprise.config` | local file path, not a URL; Tell never opens it |
| `apprise.tag` | one Apprise tag expression |
| `apprise.format` | `text`, `markdown`, `html` |
| `herdr.session` | existing session name; `default` for the default session |
| `send.via` | `hcom` or `herdr`; no default |
| `send.to` | one exact hcom name (no `@`) or Herdr agent name/pane ID |
| `send.from` | ≤64 ASCII letters, digits, `_`, `-`; not starting with `-` |
| `mcp.agent_targets` | `true` or `false` |

Step options override top-level provider settings; flag and environment
selectors override every matching step.

`send.to` refuses whitespace, lists, wildcards, tag selectors ending in `-`,
remote hcom targets, and `self`. There is no broadcast or send fallback.

Apprise `urls_env` names a variable holding URLs. Tell passes its value only
as the child's `APPRISE_URLS`, never in argv or output; unset or empty is
`not_sent/missing_urls`. It cannot combine with an effective `config` or `tag`.
Any tagged Apprise step, including single-route `notify.via: apprise`, removes
inherited `APPRISE_URLS` so the tag is honored.

Omit optional keys rather than setting them empty (`send.to` excepted).
Provider URLs and credentials are never accepted; keep them with the provider.
Removed settings are rejected: `notify.via: stdout`, `hcom.to`, `TELL_HCOM_TO`
(use `send.to`, `TELL_SEND_TO`).

## Invocation flags

| Scope | Flags |
| --- | --- |
| global (before the command) | `-C`/`--project`, `-c`/`--config`, `-j`/`--json`, `--no-color`, `--timeout`, `-h`, `-v` |
| `notify` | `--via`, `--apprise-config`, `--apprise-tag`, `--apprise-format`, `--herdr-session`, `-t`/`--title`, `-n`/`--type` |
| `send` | `--send-via`, `--to`, `--from`, `--herdr-session`, `-i`/`--intent` |
| `doctor`, `config [show\|check]`, `mcp` | every setting flag above |
| `listen` | `--all`, `--print`, `--dry-run`; sources `slack`, `github` |
| `init` | `-u`/`--user` or `-l`/`--local`; rejects `--config`, `--timeout`, and setting flags |

```sh
tell notify --title "CI" --type failure "Tests failed"   # info|success|warning|failure
tell notify Build finished                               # sends "Build finished"
tell notify - < message.txt                              # stdin only for a lone -
tell notify -- --not-a-flag                              # -- before flag-like text
tell send --intent request "Triage PRs"                  # inform (default) | request
tell send --send-via herdr --to reviewer "..."           # --via is notify-only
tell --via desktop doctor                                # rejected: routing flag before command
```

No message arguments is a usage error (exit 2), even with piped stdin. A `-`
among other words is literal. Empty messages are refused. Desktop, Herdr, and
Apprise receive the body in argv, visible to local process inspection; see
[message preparation](design.md#message-preparation).

## Route readiness

`tell doctor` checks without sending, starting services, or repairing. Run it
as the real caller, with the same environment.

```console
$ tell doctor
desktop (selected): ready
apprise: not ready
  apprise was not found on PATH; install it or choose another route

$ tell --json doctor
{"schema_version":1,"selected":"stderr","ready":true,"routes":[{"route":"stderr","ready":true,"reason":"ready"}]}
```

It checks each selected notify step, other explicitly configured routes
(Apprise settings, `herdr.session`, `notify.command`), and the send route when
`send.via`/`send.to` are set. Defaults alone add nothing; use
`tell doctor --via desktop` to check desktop.

| Route | Check | Does not verify |
| --- | --- | --- |
| stderr | nothing | — |
| desktop | `osascript` or `notify-send` on PATH | permissions, D-Bus, Focus, banners |
| command | executable resolves; never run | interpreter, its config |
| apprise | `apprise --dry-run` matches ≥1 service | credentials, reachability |
| herdr | `herdr session list --json` shows the session running | — |
| send/hcom | `hcom list --json` has one exact active name | — |
| send/herdr | `herdr agent get` reports idle or done | state at send time |

| Exit | Meaning |
| --- | --- |
| 0 | at least one notify step and any configured send route are ready |
| 1 | none ready, or config/output failed |
| 2 | usage error |

Reasons include `missing_config`, `missing_urls`, `busy` (working/unknown
agent), and `blocked` (dialog). Probes use the configured timeout, cap output at
1 MiB, fail closed on malformed output, strip `HERDR_*`, and never print URLs,
config, or provider output. JSON adds ordered `steps`, `selected_send`, and
Apprise `services`.

## Editor schemas

`tell init` adds a schema hint to new files. For existing files:

```yaml
# tell.yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/brettinternet/tell/main/tell.shared.schema.json
```

```yaml
# user config, .git/tell.yaml, or --config file
# yaml-language-server: $schema=https://raw.githubusercontent.com/brettinternet/tell/main/tell.schema.json
```

The [shared schema](../tell.shared.schema.json) rejects personal fields; the
[personal schema](../tell.schema.json) accepts everything. Replace `main` with a
commit SHA to pin. Use the comment, not a `$schema` key. Schemas assist
editors; `tell config check` remains authoritative (duplicate keys, anchors,
size, duration overflow, precedence).

## Strict YAML

One document, one mapping. Rejected: unknown or duplicate keys, aliases,
anchors, merge keys, nulls, wrong types, dotted keys, unsupported versions,
files over 1 MiB. Empty or comment-only files are empty layers. `version`
defaults to 1. Errors name the source, field, and line, never the value.

## Inspection output

```console
$ tell --json config check
{"config":{"project":"/Users/me/dev/app","timeout":"10s","notify":{"via":"stderr"},...,"origins":{...}},"schema_version":1,"valid":true}

$ tell --json --timeout 0s config check; echo $?
{"error":{"code":"config","message":"--timeout: timeout must be a positive Go duration, such as 10s"},"schema_version":1}
1
```

`--json` prints one stdout object with `schema_version: 1`; help, version, and
completion stay text. Exit 0 success, 1 config/I/O, 2 usage. Human output
quotes values so terminal escapes are inert. `notify.command` shows as
`[configured]` (omitted in JSON); `urls_env` shows only the variable name.
Delivery uses its own [outcomes](design.md#outcomes).

### Terminal styling

| Style | Used for |
| --- | --- |
| bold | headings, labels |
| green | ready, accepted |
| yellow | warnings, uncertain |
| red | errors, selected route not ready |
| default | values, paths, unavailable optional routes |

Each stream is checked separately; pipes and files get no ANSI. `--no-color`,
nonempty `NO_COLOR` (even `0`), or `TERM=dumb` disables color and bold;
`--no-color=false` does not override environment opt-outs. JSON, MCP,
completions, and message bodies are never styled. Text carries meaning without
color.

## Initialization

| Command | Creates | Mode |
| --- | --- | --- |
| `tell init` | `tell.yaml` (shared fields) | default |
| `tell init -u` | user config, with parent dirs (personal routing) | 0600 |
| `tell init -l` | `tell.yaml` in the common Git directory (personal routing) | 0600 |

```console
$ tell init
Created "/Users/me/dev/app/tell.yaml"
Next: commit tell.yaml. Run tell init --user to configure personal routing.

$ tell init
Kept existing "/Users/me/dev/app/tell.yaml" (not overwritten)
Next: commit tell.yaml. Run tell init --user to configure personal routing.

$ tell init --user
Created "/Users/me/.config/tell/config.yaml"
Next: set notify.via in this file, then run tell doctor.
```

Existing files are kept byte-for-byte; symlinks and directories are refused.
`--local` fails outside a Git checkout; `--user` cannot combine with `--local`
or `--project`. Init never edits `.gitignore` or reads provider config.
