# Escalate until acknowledged

Two patterns, which combine: the provider repeats one alert (Pushover
emergency priority), or Noon sends louder [timed tiers](#timed-tiers-with-noon)
until you acknowledge. Tell stores no state in either; the scripts keep the
Pushover receipt in a file.

Pushover emergency alerts repeat on your phone until you acknowledge them in
the Pushover app, or until they expire. Tell's direct `pushover` route sends
them and returns the receipt, so a later tier can ask once whether you
acknowledged, and `ack.sh` can stop the repeats by tag.

Use the direct route, not Apprise or Shoutrrr, for this. Neither can expose a
receipt: Shoutrrr's `Send` returns only errors, and the Apprise CLI prints no
provider response. Shoutrrr `v0.22.1` also drops `priority=2`, so its alert
goes out at normal priority, never repeats, and still reports `accepted`.

## Setup

```sh
cp examples/escalate/config.yaml ~/.config/tell/critical.yaml
export PUSHOVER_TOKEN=…   # application token; from a secret manager, not a file Tell reads
export PUSHOVER_USER=…    # user or group key
tell --config ~/.config/tell/critical.yaml doctor   # checks the variables; no send
```

```yaml
# ~/.config/tell/critical.yaml
version: 1
notify:
  via: pushover
pushover:
  token_env: PUSHOVER_TOKEN
  user_env: PUSHOVER_USER
  priority: emergency   # repeats until acknowledged in the app or expired
  retry: 2m             # at least 30s
  expire: 1h            # at most 3h
```

Send critical alerts through that config; everything else keeps your normal
routes:

```sh
tell --config ~/.config/tell/critical.yaml --json notify --pushover-tag inc-db "Database down"
# {"outcome":"accepted","receipt":"r…","route":"pushover",…}
tell --config ~/.config/tell/critical.yaml pushover status r…        # exit 0 acknowledged, 3 not
tell --config ~/.config/tell/critical.yaml pushover cancel --tag inc-db
```

A team on-call service (PagerDuty, Opsgenie) through Apprise runs its own
escalation policy; put it before Pushover in a cascade if you have one.

## Accepted is not acknowledged

`accepted` (exit 0) means Pushover accepted the alert. It does not mean you saw
it. Repeats and acknowledgement happen inside Pushover; Tell exits after the
first send and never waits or polls. Run the command once: each run starts
another emergency alert.

| Tell outcome | Means |
| --- | --- |
| `accepted` (0) | Pushover will repeat it until acknowledged, expired, or cancelled; `--json` has the `receipt` |
| `not_sent` (3) | nothing reached Pushover; safe to rerun |
| `failed` (4), `uncertain` (5) | it may have arrived; check the Pushover app before rerunning |

`--pushover-callback URL` asks Pushover to call your URL on acknowledgement.
Tell passes it literally; receiving it is up to you.

## Timed tiers with Noon

Pushover now, then louder elsewhere unless someone acknowledges. Noon owns the
timers as temporary jobs; Tell only delivers each tier. Noon is one choice:
any scheduler that meets the [tier contract](../../docs/design.md#escalation)
works, and Tell itself never calls it.

```sh
./raise.sh 'db-primary down' 'Database down'   # tiers 2-3 scheduled; tier 1 Pushover now
./ack.sh 'db-primary down'                     # stop the Pushover repeats and every unsent tier
```

Needs Noon with temporary jobs (tested with 0.15.0), a running scheduler
(`noon serve` or `noon service`), `jq`, and `sha256sum` or `shasum`. Tier jobs
run in Noon's user project, so they need the
[service setup](../noon/README.md#service-setup): `tell` on PATH, your Tell
config, and the Pushover variables. The user project's directory must exist:
`mkdir -p ~/.config/noon`.

Edit the policy at the top of [raise.sh](raise.sh), or override it in the
environment:

```sh
CRITICAL=~/.config/tell/critical.yaml                          # tier 1: Pushover emergency
TIER1_NOTIFY='--type failure'                                  # sent at once
TIER2='10m --type failure'                                     # DELAY [notify options]
TIER3='30m --type failure --via apprise --apprise-tag phone'
LIFETIME=2h                                                    # jobs end on their own
PUSHOVER_CALLBACK=https://example.com/ack                      # optional
```

Later tiers use your normal config, so they reach a different route, such as a
[fallback cascade](../fallback/README.md) or another person.

| Step | What runs |
| --- | --- |
| raise | `noon --user add inc-<hash>-tN --temporary --key inc-<hash> --in DELAY --for LIFETIME -- tier.sh … tell notify …`, then `tell --json notify --pushover-tag inc-<hash>`; the receipt goes to a per-raise file under `$XDG_STATE_HOME/tell-escalate/` |
| raise again | same key returns `created: false`; no new jobs, tier 1 not resent |
| tier due | [tier.sh](tier.sh) runs `tell pushover status RECEIPT` once; acknowledged (exit 0) skips the tier, anything else sends it |
| ack | `tell pushover cancel --tag inc-<hash>`, then `noon --user cancel` every active `inc-<hash>-t*` job |
| nobody acks | Pushover stops at `expire`; jobs end at `LIFETIME`; Noon deletes them 24h later |

Later tiers are scheduled before tier 1 is sent, so if the raising process
dies they still fire. A missing receipt or an unreachable Pushover sends the
tier: doubt escalates. Noon job names allow only `[a-z0-9_-]`, so the incident
key is hashed: the first 12 hex characters of its SHA-256, which is also the
Pushover tag. Every tier is argv, never a shell string, so titles and keys
cannot inject commands. An incident counts as raised until it is acknowledged
with `ack.sh` or `LIFETIME` passes; raising it after `ack.sh` starts a new
escalation.

| Case | Behavior |
| --- | --- |
| acknowledged in the Pushover app | repeats stop; later tiers skip themselves |
| `ack.sh` before a tier is due | Pushover repeats stop; the tier never runs |
| `ack.sh` while a tier runs | Noon sends TERM; that alert is uncertain, may have arrived |
| ack after a tier is delivered | cannot be recalled |
| Noon down across a due time | the tier runs late, once, when Noon restarts |
| Noon down past `LIFETIME` | queued tiers are cancelled; nothing is sent (per Noon docs) |
| two machines | each has its own Noon and receipt file; `ack.sh` cancels Pushover everywhere, Noon jobs locally |

To acknowledge at the keyboard, show an [alert](../../docs/design.md#alert-delivery)
after raising; clicking Acknowledge runs `ack.sh`:

```sh
./raise.sh 'db-primary down' 'Database down'
tell notify --via alert --title 'Database down' 'db-primary down' && ./ack.sh 'db-primary down'
```

Set `LIFETIME` to how long an alert stays meaningful, since a late tier still
sends.
