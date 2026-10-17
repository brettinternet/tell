# GitHub Actions: tell the team when the nightly breaks

Scheduled workflows fail quietly: nobody is watching the Actions tab at 6am.
[notify-on-failure.yaml](notify-on-failure.yaml) posts a pointer to the failed
run through Apprise, using the same `tell notify` you use locally.

```text
acme/app nightly failed
Branch: main
Commit: 3f9c2e1
Run: https://github.com/acme/app/actions/runs/1234567890
```

## Setup

1. Add a repository secret `APPRISE_URLS` with one or more Apprise URLs,
   for example `slack://T000/B000/XXXX/#dev-alerts` or a Discord webhook.
2. Copy the workflow to `.github/workflows/nightly.yaml` and replace
   `make test` with your checks.

Routing comes from the job's environment (`TELL_NOTIFY_VIA=apprise`), the CI
equivalent of personal config. Apprise reads `APPRISE_URLS` itself; Tell
never handles the webhook.

## Notes

- Context is passed through `env:` instead of `${{ }}` inside `run:`, so a
  branch name cannot inject shell.
- The step only runs `if: failure()` and installs Tell and Apprise only then.
- A delivery problem becomes a workflow warning, not a second failure.
- Every failing run notifies. For transition-only alerts across runs, you need
  state between runs; locally, see [scheduled-checks](../scheduled-checks/README.md).
- To have an agent investigate instead, see [ci-to-agent](../ci-to-agent/README.md).
