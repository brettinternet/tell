# Task runners: notify from Taskfile and Make

Put the notification next to the slow task so every caller gets it: you,
an agent running `task e2e`, or a scheduler.

## Taskfile

[Taskfile.yml](Taskfile.yml) uses `defer`, which runs even when a command
fails. `EXIT_CODE` is empty on success.

```yaml
build:release:
  cmds:
    - defer: >-
        tell notify
        --type {{if .EXIT_CODE}}failure{{else}}success{{end}}
        --title "task {{.TASK}}"
        -- "{{if .EXIT_CODE}}Failed with exit {{.EXIT_CODE}}{{else}}Finished{{end}}"
        || true
    - go build -trimpath -o dist/app ./cmd/app
```

The `e2e` task notifies only on failure and points at the report instead of
pasting test output.

## Make

[Makefile](Makefile) adds a pattern rule. Any target gains a `-notify` variant:

```sh
make test-notify
make docker-image-notify
```

## Notes

- `|| true` keeps a delivery problem from failing the build. Task still exits
  with the build's status.
- Shared task files should not choose routes. Each person's config decides
  whether `tell notify` prints to stderr, pops a banner, or pings a phone.
