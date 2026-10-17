# Wait for something, then tell me

Small pollers for the "is it ready yet?" moments of local development.
Each exits after one notification.

| Script | Waits for | Example |
| --- | --- | --- |
| [wait-url.sh](wait-url.sh) | an HTTP endpoint to answer | dev server, `docker compose up`, a port-forward |
| [wait-pr-checks.sh](wait-pr-checks.sh) | all checks on a pull request | CI after `git push` |
| [wait-pid.sh](wait-pid.sh) | a process you already started | a migration in another terminal |

```sh
docker compose up -d && examples/wait-for/wait-url.sh http://localhost:8080/healthz 180
git push && examples/wait-for/wait-pr-checks.sh &
examples/wait-for/wait-pid.sh "$(pgrep -n -f 'rails db:migrate')"
```

```text
Ready            http://localhost:8080/healthz is up after 37s
Checks failed    lint, test (ubuntu-latest)
                 https://github.com/acme/app/pull/128
```

`wait-pr-checks.sh` needs `gh` authenticated for the repository. It reports
failing check names and the PR URL, never logs. To have an agent investigate
the failure, see [ci-to-agent](../ci-to-agent/README.md).

## Notes

- These scripts exit nonzero on failure or timeout, so they also work as
  gates: `wait-url.sh "$URL" && bun run e2e`.
- Polling belongs in the caller. Tell itself never waits or retries.
