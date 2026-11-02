# Escalate until acknowledged with Pushover emergency priority

Pushover emergency alerts repeat on your phone until you acknowledge them in
the Pushover app, or until they expire. Apprise sends them, so Tell needs no
escalation logic: the escalation is one Apprise step at the end of a notify
cascade. The Pushover URL and keys live only in Apprise's config.

## Setup

```sh
pipx install apprise        # or: uv tool install apprise
mkdir -p ~/.config/apprise
cp examples/escalate/apprise.yaml ~/.config/apprise/escalate.yaml
chmod 600 ~/.config/apprise/escalate.yaml   # then replace the placeholders
cp examples/escalate/config.yaml ~/.config/tell/critical.yaml
tell --config ~/.config/tell/critical.yaml doctor   # no send
```

| File | Owns |
| --- | --- |
| [apprise.yaml](apprise.yaml) | Pushover keys and emergency settings, tag `escalate` |
| [config.yaml](config.yaml) | Tell cascade: on-call service, then Pushover emergency |

Send critical alerts through that config; everything else keeps your normal
routes:

```sh
tell --config ~/.config/tell/critical.yaml notify --type failure "Backups failed 3 nights running"
```

```yaml
# ~/.config/tell/critical.yaml
version: 1
notify:
  via:
    - apprise: {tag: oncall}     # team on-call service, if apprise.yaml has one
    - apprise: {tag: escalate}   # Pushover emergency: repeats until acknowledged
  fallback_on: [not_sent]
apprise:
  config: ../apprise/escalate.yaml
```

With no `oncall` service, the first step reports `not_sent/no_match` and the
cascade moves on to Pushover. A PagerDuty or Opsgenie URL tagged `oncall`
works the same way: the provider runs its own escalation policy. Delete the
first step if you only page yourself.

## Pushover settings

```yaml
# ~/.config/apprise/escalate.yaml
urls:
  - pover://USER_KEY@APP_TOKEN?priority=emergency&interval=120&expire=3600:
      - tag: escalate
```

| Parameter | Meaning | Apprise default | Limits |
| --- | --- | --- | --- |
| `priority=emergency` | repeat until acknowledged (`2` also works) | `normal` | — |
| `interval` | seconds between repeats; Pushover's API calls it `retry` | 900 | at least 30 |
| `expire` | seconds before Pushover stops repeating | 3600 | 0–10800 |

Apprise ignores `retry=` in the URL, so a misspelled interval silently falls
back to 15 minutes. An out-of-range value makes Apprise drop the URL, and the
step reports `not_sent/no_match`. Check the parsed URL without sending:

```sh
apprise --dry-run --config ~/.config/apprise/escalate.yaml --tag escalate -t test -b test
```

Instead of a tag, a step can select the destination with
`apprise: {config: FILE}` or, on CI, `apprise: {urls_env: ESCALATE_APPRISE_URLS}`
with the `pover://` URL in a secret.

## Accepted is not acknowledged

`accepted` (exit 0) means Pushover accepted the alert. It does not mean you saw
it or acknowledged it. Repeats and acknowledgement happen inside Pushover; Tell
exits after the first send, does not poll the Pushover receipt, and never
learns whether you acknowledged. Run the command once: each run starts another
emergency alert.

| Tell outcome | Means |
| --- | --- |
| `accepted` (0) | Pushover will repeat it until acknowledged or expired |
| `not_sent` (3) | nothing reached Pushover; safe to rerun |
| `failed` (4), `uncertain` (5) | it may have arrived; check the Pushover app before rerunning |

Tell has no timed escalation tiers or acknowledgement tracking of its own; see
[docs/design.md](../../docs/design.md#escalation).
