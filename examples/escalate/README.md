# Escalate until acknowledged

Two patterns, which combine: the provider repeats one alert (Pushover
emergency priority), or Noon sends louder [timed tiers](#timed-tiers-with-noon)
until you run `ack.sh`. Tell stores no state in either.

Pushover emergency alerts repeat on your phone until you acknowledge them in
the Pushover app, or until they expire. Apprise sends them, so Tell needs no
escalation logic: the escalation is one Apprise step at the end of a notify
cascade. The Pushover URL and keys live only in Apprise's config.

Don't use the built-in Shoutrrr route for this. Shoutrrr `v0.22.1` drops
`priority=2` (it lacks Pushover's required `retry` and `expire`), so the alert
goes out at normal priority, never repeats, and still reports `accepted`.

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

## Timed tiers with Noon

Notify now, then louder later unless someone acknowledges. Noon owns the
timers as temporary jobs; Tell only delivers each tier. Noon is one choice:
any scheduler that meets the [tier contract](../../docs/design.md#escalation)
works, and Tell itself never calls it.

```sh
./raise.sh 'db-primary down' 'Database down'   # tier 1 now; tiers 2-3 scheduled
./ack.sh 'db-primary down'                     # cancel every unsent tier
```

Needs Noon with temporary jobs (tested with 0.15.0), a running scheduler
(`noon serve` or `noon service`), `jq`, and `sha256sum` or `shasum`. Tier jobs
run in Noon's user project, so they need the
[service setup](../noon/README.md#service-setup): `tell` on PATH and your Tell
config. The user project's directory must exist: `mkdir -p ~/.config/noon`.

Edit the policy at the top of [raise.sh](raise.sh), or override it in the
environment:

```sh
TIER1_NOTIFY='--type warning'                                  # sent at once
TIER2='10m --type failure'                                     # DELAY [notify options]
TIER3='30m --type failure --via apprise --apprise-tag phone'
LIFETIME=2h                                                    # jobs end on their own
```

Without `--via`, a tier uses your configured route, which may be a
[fallback cascade](../fallback/README.md) ending in the Pushover step above.

| Step | Noon |
| --- | --- |
| raise | `noon --user add inc-<hash>-tN --temporary --key inc-<hash> --in DELAY --for LIFETIME -- tell notify …` |
| raise again | same key returns `created: false`; no new jobs, tier 1 not resent |
| tier due | Noon runs `tell notify` once |
| ack | `noon --user cancel` every active `inc-<hash>-t*` job |
| nobody acks | jobs end at `LIFETIME`; Noon deletes them 24h later |

Noon job names allow only `[a-z0-9_-]`, so the incident key is hashed: the
first 12 hex characters of its SHA-256. Every tier is argv, never a shell
string, so titles and keys cannot inject commands. An incident counts as raised
until it is acknowledged or `LIFETIME` passes, even after its last tier is
delivered; raising it after `ack.sh` starts a new escalation.

| Case | Behavior |
| --- | --- |
| ack before a tier is due | the tier never runs |
| ack while a tier runs | Noon sends TERM; that alert is uncertain, may have arrived |
| ack after a tier is delivered | cannot be recalled |
| Noon down across a due time | the tier runs late, once, when Noon restarts |
| Noon down past `LIFETIME` | queued tiers are cancelled; nothing is sent (per Noon docs) |
| two machines | each has its own Noon; acknowledgement is local only |

To acknowledge at the keyboard, show an [alert](../../docs/design.md#alert-delivery)
after raising; clicking Acknowledge cancels the later tiers:

```sh
./raise.sh 'db-primary down' 'Database down'
tell notify --via alert --title 'Database down' 'db-primary down' && ./ack.sh 'db-primary down'
```

Acknowledgement is a local command or click, not a reply from your phone or Slack, and
does not acknowledge a Pushover emergency alert already sent. Set `LIFETIME` to
how long an alert stays meaningful, since a late tier still sends.
