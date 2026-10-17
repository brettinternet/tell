# Scheduled checks: alert on change, not on every run

A check that runs every five minutes and notifies on every failure trains you
to ignore it. [check-transition.sh](check-transition.sh) runs any command and
notifies only when its result flips:

```text
Check: staging    staging is failing on mbp. Check: curl -fsS ... https://staging.example.com/healthz
Check: staging    staging recovered on mbp.
```

Tell keeps no state, so the wrapper does: one file per check under
`~/.local/state/tell-checks/`.

## Checks

Any command that exits 0 when healthy works. Three helpers:

| Script | Healthy when |
| --- | --- |
| [disk-below.sh](disk-below.sh) `90 /` | the mount is under 90% used |
| [fresher-than.sh](fresher-than.sh) `1560 /srv/backups` | something under the path changed in the last 26h |
| [cert-valid-for.sh](cert-valid-for.sh) `14 example.com` | the TLS certificate is valid for 14 more days |

```sh
cp examples/scheduled-checks/*.sh ~/.local/bin/ && chmod +x ~/.local/bin/*.sh
check-transition.sh staging curl -fsS --max-time 10 -o /dev/null https://staging.example.com/healthz
check-transition.sh ollama curl -fsS -o /dev/null http://localhost:11434/api/version
check-transition.sh docker docker info
check-transition.sh disk disk-below.sh 90 /
```

## Schedule it

| Scheduler | File | Desktop banners? |
| --- | --- | --- |
| cron | [crontab](crontab) | Linux needs `DBUS_SESSION_BUS_ADDRESS`; macOS: use launchd |
| launchd (macOS) | [com.example.check-staging.plist](com.example.check-staging.plist) | yes, as a LaunchAgent |
| systemd (Linux) | [check-staging.service](check-staging.service) + [.timer](check-staging.timer) | yes, as a user unit |
| Noon | [noon](../noon/README.md) | see its service setup |

Schedulers do not inherit your shell. Set `PATH` so `tell` and the provider
resolve, and run `tell doctor` in the same environment once.

## What happens when delivery fails

| Tell exit | State | Next run |
| --- | --- | --- |
| 0 accepted | updated | quiet until the next flip |
| 1–3 nothing sent | kept | reports the same transition again |
| 4 failed, 5 uncertain | updated | quiet; it may have arrived, so no blind resend |

## Notes

- Runs that overlap can race on the state file. Keep checks shorter than
  their interval, or use a scheduler with overlap control.
- Check output is discarded and never forwarded. The message names the
  command so you can rerun it.
- `cert-valid-for.sh` in the crontab is used without the wrapper on purpose:
  a daily reminder until the certificate is renewed.
