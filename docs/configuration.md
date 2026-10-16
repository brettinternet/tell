# Configuration

Everything on this page is implemented configuration behavior. Delivery adapters
are not implemented; selecting a provider does not execute or verify it.

## Sources

Lowest to highest priority:

1. Built-in defaults.
2. Optional `/etc/tell/config.yaml`.
3. `$XDG_CONFIG_HOME/tell/config.yaml`, defaulting to `~/.config/tell/config.yaml`.
4. Shared project `tell.yaml`.
5. Private project `.tell.yaml`.
6. `TELL_*` setting environment variables.
7. Explicit CLI flags.

Mappings merge by field. Each field records its winning origin: default, absolute
file path, environment name, or flag name. There are no list-valued settings yet.
Missing optional files are fine; unreadable files, directories, dangling symlinks,
and malformed lower-priority files are errors even if a later layer overrides
their values. No settings directory is created by inspection.

`--config FILE` / `TELL_CONFIG` replaces **all automatic file sources**,
including system config. The explicit file must exist. Defaults and setting
env/flags still apply; use a controlled environment for reproducible CI.
HOME and XDG_CONFIG_HOME are not consulted, so services without HOME can use it.

`--system-config FILE` / `TELL_SYSTEM_CONFIG` substitutes an explicit, required
system-defaults file for the optional /etc path. It has no effect when --config
selects a single file. This supports alternate installations and isolated tests.
System values are defaults, not enforced administrator policy.

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
discovering or inspecting config. Future delivery uses repository routing policy,
so review repository config before sending messages from untrusted checkouts.

## Settings

| YAML field | Default | Environment | Flag |
| --- | --- | --- | --- |
| timeout | 10s | TELL_TIMEOUT | --timeout |
| notify.via | stdout | TELL_NOTIFY_VIA | --via |
| apprise.config | unset | TELL_APPRISE_CONFIG | --apprise-config |
| apprise.tag | unset | TELL_APPRISE_TAG | --apprise-tag |
| apprise.format | unset | TELL_APPRISE_FORMAT | --apprise-format |
| herdr.session | unset | TELL_HERDR_SESSION | --herdr-session |
| hcom.to | unset | TELL_HCOM_TO | --to |

The table uses dotted names for reference; YAML must use nested mappings:

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

- timeout is a positive Go duration; zero and indefinite waits are refused.
- notify.via is stdout, desktop, apprise, or herdr.
- apprise.config is a local file path, never a credential-bearing URL. Tell
  does not read it or check that it exists during config inspection.
- apprise.tag is a single expression preserved for Apprise.
- apprise.format is text, markdown, or html.
- herdr.session and hcom.to are explicit identifiers, not executable commands.
- Optional settings are unset until supplied. An explicitly empty value is an
  error; omit it instead. A flag replaces an env value before validation.
- No arbitrary argv, provider URLs, plugin paths, or credential fields are accepted.

Native APPRISE_* forwarding belongs to the future delivery adapter; currently
Tell does not interpret or display those variables.

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
remain standard text even with --json.

## Initialization

`tell init` creates tell.yaml with commented examples; --local selects .tell.yaml
with mode 0600. Existing regular files are kept byte-for-byte, including malformed
files. Existing symlinks/directories are refused. Init never changes .gitignore;
add `.tell.yaml` yourself. It does not materialize inherited values or read
provider configuration, and refuses --config or setting overrides rather than
silently ignoring them. Project selection still applies.

Keep .tell.yaml and provider credentials out of Git. A shared repository config
should normally choose tags, not contain personal service destinations.
