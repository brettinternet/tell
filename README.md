# Tell

**Small delivery plumbing for humans and agents.**

A Go CLI intended to route notifications through Apprise and Herdr, and messages
through hcom. Repo scripts own workflows; Noon owns scheduling and queued jobs.
Tell executes one invocation and exits.

Bootstrap in progress: tooling and Backlog.md are established. Configuration
inspection and initialization are being implemented; delivery adapters are
tracked separately. Nothing sends notifications or starts a service yet.

## Development

```sh
mise trust
mise exec -- task init
```

See [development](docs/development.md). MIT licensed.
