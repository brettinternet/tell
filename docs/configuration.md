# Configuration

Everything on this page is implemented configuration behavior. `notify` delivers
via stderr or optional external Apprise. Inspection never executes a provider.
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
file path, environment name, or flag name. There are no list-valued settings yet.
Missing optional files are fine; unreadable files, directories, dangling symlinks,
and malformed lower-priority files are errors even if a later layer overrides
their values. No settings directory is created by inspection.

`--config FILE` / `TELL_CONFIG` replaces **all automatic file sources**.
The explicit file must exist. Defaults and setting
env/flags still apply; use a controlled environment for reproducible CI.
HOME and XDG_CONFIG_HOME are not consulted, so services without HOME can use it.

Shared `tell.yaml` rejects `notify.via`, `apprise.config`, `herdr.session`, and
`hcom.to`, with a source/line diagnostic directing you to a personal source.
Set routing in user config, private `.tell.yaml`, an explicit `--config` file,
environment, or flags. Even a higher-priority override cannot make routing in
automatically loaded shared config valid. Explicitly selecting `tell.yaml` with
`--config` trusts that file as a personal source. There is no system config layer.

`XDG_CONFIG_HOME` must be absolute when nonempty. Paths in YAML are relative to
the file that declared them, resolved **before merging**. Paths from flags/env
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
- notify.via is stderr or apprise. desktop and herdr are accepted configuration
  values for planned routes; invoking them returns not_sent, never success.
  The former bootstrap value stdout is no longer accepted.
- apprise.config is a local file path, never a credential-bearing URL. Tell
  does not read it or check that it exists during config inspection.
- apprise.tag is a single expression preserved for Apprise.
- apprise.format is text, markdown, or html.
- herdr.session and hcom.to are explicit identifiers, not executable commands.
- Optional settings are unset until supplied. An explicitly empty value is an
  error; omit it instead. A flag replaces an env value before validation.
- No arbitrary argv, provider URLs, plugin paths, or credential fields are accepted.

Apprise inherits native APPRISE_* environment variables; Tell does not interpret
or display them. Tell always supplies --body, --title, and --notification-type.
Unset config, tag, and format options are omitted so Apprise's native behavior
applies (without --tag, only untagged services are notified). No URL arguments
are accepted. Provider stdin is closed; stdout/stderr are discarded.

`tell notify MESSAGE` never reads stdin; `tell notify -` explicitly reads it.
`--title` defaults to the selected project directory name. `--type` is info
(default), success, warning, or failure. These are invocation flags, not YAML
settings. Empty or whitespace-only messages are refused. Titles and bodies are
normalized, scanned with embedded Gitleaks default rules, then truncated; see
[message preparation](design.md#message-preparation). JSON results include a
`redactions` count (merged replaced occurrences across both fields). Scanning
is offline and best-effort; undetected secrets can still be delivered.
Apprise receives the body via argv, so local process inspection may reveal it.
Do not include secrets in notifications.

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
terminal control sequences from being interpreted.

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
