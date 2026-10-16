# Configuration

Everything on this page is implemented configuration behavior. `notify` delivers
via stderr, native desktop, optional external Apprise or Herdr, or a personal command. Inspection never executes a provider.
Provider acceptance is not human acknowledgment.

## Sources

Lowest to highest priority:

1. Built-in defaults.
2. `$XDG_CONFIG_HOME/tell/config.yaml`, defaulting to `~/.config/tell/config.yaml`.
3. Shared project `tell.yaml` (shared fields only).
4. Private project `.tell.yaml`.
5. `TELL_*` setting environment variables.
6. Explicit CLI flags.

Mappings merge by field. Each field records its winning origin: default, absolute
file path, environment name, or flag name. The notify.command argv list is replaced
as a whole by a higher layer, never appended or merged by index.
Missing optional files are fine; unreadable files, directories, dangling symlinks,
and malformed lower-priority files are errors even if a later layer overrides
their values. No settings directory is created by inspection.

`--config FILE` / `TELL_CONFIG` replaces **all automatic file sources**.
The explicit file must exist. Defaults and setting
env/flags still apply; use a controlled environment for reproducible CI.
HOME and XDG_CONFIG_HOME are not consulted, so services without HOME can use it.

Shared `tell.yaml` rejects `notify.command`, `notify.via`, `apprise.config`, `herdr.session`, and
`hcom.to`, with a source/line diagnostic directing you to a personal source.
Set routing in user config, private `.tell.yaml`, an explicit `--config` file,
environment, or flags. Even a higher-priority override cannot make routing in
automatically loaded shared config valid. Explicitly selecting `tell.yaml` with
`--config` trusts that file as a personal source. There is no system config layer.

`XDG_CONFIG_HOME` must be absolute when nonempty. Paths in YAML are relative to
the file that declared them, resolved **before merging** (except command argv,
which is passed literally; see the command contract below). Paths from flags/env
are relative to the invocation directory. No tilde, shell, or variable expansion
occurs in YAML. For example, user-config `apprise.config: ../apprise/apprise.yaml`
resolves under the user config directory, not the current repository.

## Project discovery

`--project DIR` / `-C DIR` / `TELL_PROJECT` selects exactly that directory.
Otherwise walk upward from the current directory to the nearest directory
containing tell.yaml, .tell.yaml, or .git. A worktree's .git file counts as a
boundary. Never inherit configuration above that boundary. With no marker, use
the invocation directory. Selected project symlinks are canonicalized.

--config does not change project selection. No repository code is executed while
discovering or inspecting config. Shared repositories cannot select delivery
destinations; private configuration and explicit invocation inputs are trusted.

## Settings

| YAML field | Default | Environment | Flag |
| --- | --- | --- | --- |
| timeout | 10s | TELL_TIMEOUT | --timeout |
| notify.via | stderr | TELL_NOTIFY_VIA | --via |
| notify.command | unset | — | — |
| secrets.policy | redact | — | — |
| apprise.config | unset | TELL_APPRISE_CONFIG | --apprise-config |
| apprise.tag | unset | TELL_APPRISE_TAG | --apprise-tag |
| apprise.format | unset | TELL_APPRISE_FORMAT | --apprise-format |
| herdr.session | unset | TELL_HERDR_SESSION | --herdr-session |
| hcom.to | unset | TELL_HCOM_TO | --to |

The table uses dotted names for reference; YAML must use nested mappings.
This complete example belongs in **user or private config**, not shared tell.yaml:

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
hcom:
  to: reviewer
```

- secrets.policy is `redact` (replace detected secrets with `[REDACTED]`) or
  `reject` (exit 1, reason `secret_detected`, before dispatch). It is accepted
  only in user config, private `.tell.yaml`, or an explicit `--config` file;
  shared `tell.yaml` rejects it, even when another layer overrides it. No
  environment or flag override, disable switch, or custom rules are supported.
- timeout is a positive Go duration; zero and indefinite waits are refused.
- notify.via is stderr, desktop, apprise, command, or herdr.
  Herdr requires an explicit herdr.session for delivery (including `default`).
  The former bootstrap value stdout is no longer accepted.
- apprise.config is a local file path, never a credential-bearing URL. Tell
  does not read it or check that it exists during config inspection.
- apprise.tag is a single expression preserved for Apprise.
- apprise.format is text, markdown, or html.
- herdr.session and hcom.to are explicit identifiers, not executable commands.
- Optional settings are unset until supplied. An explicitly empty value is an
  error; omit it instead. A flag replaces an env value before validation.
- notify.command is a non-empty argv list of strings, with a non-blank executable
  and no NUL bytes. Empty arguments after the executable are allowed. Only user
  config, private .tell.yaml, and --config accept it; no env/flag argv form exists.
  Selecting command without this setting fails before dispatch.
- No provider URL, plugin path, or credential fields are accepted. Keep credentials
  with your provider or executable, not in command arguments.

Apprise inherits native APPRISE_* environment variables; Tell does not interpret
or display them. For delivery, Tell always supplies --body, --title, and --notification-type.
Unset config, tag, and format options are omitted so Apprise's native behavior
applies (without --tag, only untagged services are notified). No URL arguments
are accepted. Apprise stdin is closed; stdout/stderr are discarded.

`tell notify MESSAGE` never reads stdin; `tell notify -` explicitly reads it.
`--title` defaults to the selected project directory name. `--type` is info
(default), success, warning, or failure. These are invocation flags, not YAML
settings. Empty or whitespace-only messages are refused. Titles and bodies are
normalized, scanned with embedded Gitleaks default rules, then truncated; see
[message preparation](design.md#message-preparation). JSON results include a
`redactions` count (merged replaced occurrences across both fields). Scanning
is offline and best-effort; undetected secrets can still be delivered.
Desktop providers, Herdr, and Apprise receive the body via argv, so local process inspection may reveal it.
Do not include secrets in notifications.

For the personal command route, see the [exact contract and worked example](design.md#command-delivery).

## Route readiness

After configuration, run `tell doctor`, or `tell --json doctor` for automation.
Use the same user, environment, and config as the intended caller. It never
sends a test message, starts services, installs providers, or repairs settings.

The selected route is checked first, followed by other explicitly configured
routes: any Apprise setting, herdr.session, notify.command, or hcom.to.
Defaults alone do not add routes. hcom remains unsupported and is reported as
such when configured. Use `--via desktop` to check desktop availability.

- stderr needs no external executable.
- desktop checks for osascript on macOS or notify-send on Linux; other platforms
  are unsupported. It does not execute them or verify notification permissions,
  D-Bus connectivity, Focus settings, or visible banners.
- command checks executable resolution/permissions only; it never runs the
  command or exposes its path/arguments. This cannot verify interpreter availability
  or the command's own provider configuration.
- Apprise runs `--dry-run` with the effective config, tag, and format, inherits
  native APPRISE_* settings, and reports only the matching service count.
  Zero matches or a failed dry run are not ready. No URLs or provider diagnostics
  are printed. Apprise itself reads its configuration.
- Herdr runs `session list --json` with inherited HERDR_* variables removed and
  requires an exact, running configured session. It does not attach or start one.

Each probe uses the configured timeout. Captured output is bounded to 1 MiB;
oversized or malformed responses fail closed. Readiness is a point-in-time
setup check, not a delivery guarantee or human acknowledgment.

JSON is one schema_version 1 object with selected, ready, and routes.
Each route has route, ready, and a fixed reason; Apprise also includes services
when its dry-run output was collected. No raw configuration is included.
Exit 0 means the selected route is ready, even if another configured route
is not; exit 1 means it is not ready or configuration/output failed. Invalid
invocations exit 2. Bare `tell` and `config check` still execute no providers.

## Strict YAML and output

Only one mapping/document is accepted. Reject unknown/duplicate keys, aliases,
anchors, merge keys, nulls, wrong types, unsupported versions, and files larger
than 1 MiB. An empty, whitespace-only, or comment-only file or an empty mapping
is accepted as an empty layer.
Version is optional and defaults to 1. Errors name the source; semantic errors
also name the field/line. Parser diagnostics never echo raw setting values.

`tell`, `tell config`, and `tell config show` inspect effective settings and
origins; `tell config check` validates the same snapshot. Neither executes
providers or reads provider credentials. Human settings are quoted to keep
terminal control sequences from being interpreted. Raw notify.command argv is
never displayed: human inspection shows only `[configured]` (or empty if unset),
JSON omits the command value, and origins still reports its winning source.

--json produces one stdout object with schema_version: 1. Successful inspection
includes config; check also includes valid: true. Failures include an error with
code and message. Human errors go to stderr. Exit codes: 0 success, 2 usage
error, 1 configuration/I/O failure. Help, version, and shell-completion output
remain standard text even with --json. Delivery has its own
[outcomes and exit codes](design.md#outcomes), including uncertain delivery.
For notify JSON, route is an empty string when parsing/configuration failed
before a route could be resolved; errors never echo invocation input.

## Initialization

`tell init` creates tell.yaml with commented shared-field examples; --local selects
.tell.yaml with personal routing examples and mode 0600. `tell init --user` creates
`$XDG_CONFIG_HOME/tell/config.yaml` (default `~/.config/tell/config.yaml`) with mode
0600 and commented personal routing examples, creating parent directories as
needed. --user cannot be combined with --local or --project. Existing regular files are kept byte-for-byte, including malformed
files. Existing symlinks/directories are refused. Init never changes .gitignore;
add `.tell.yaml` yourself. It does not materialize inherited values or read
provider configuration, and refuses --config or setting overrides rather than
silently ignoring them. Project selection applies to project init only.

Keep .tell.yaml and provider credentials out of Git. A shared repository config
may choose tags and format, but must not contain personal routing fields.
