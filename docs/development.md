# Development

Go 1.27.2, urfave/cli v3, and strict YAML, matching Noon. Tooling follows the
sibling hum, bump, and project repositories, trimmed to a Go-only CLI.

## Setup

```sh
mise trust && mise exec -- task init   # pinned tools, Go modules, Lefthook hooks; starts nothing
```

## Tasks

| Task | Does |
| --- | --- |
| `task cli:build` | build `bin/tell` |
| `task cli:run -- --help` | run from source |
| `task check` | gofmt, vet, Staticcheck, changelog generation |
| `task test` / `task race` | unit tests, with race detector |
| `task smoke` | build, then test the binary in an isolated environment |
| `task security` | Gitleaks over Git history, govulncheck |
| `task ci` | all of the above; CI runs it on Linux and macOS |
| `task check:staged` | format and re-stage Go, scan the staged snapshot |
| `task changelog` | write CHANGELOG.md from Conventional Commits |
| `task backlog` / `task backlog:next` | ready, unclaimed work |

Before committing: stage only intended files, run `task check:staged`, use a
Conventional Commit. Never push without instruction. Releases are not wired up.

## Layout

| Path | Owns |
| --- | --- |
| `cmd/tell` | process boundary, signals, linker-injected build metadata |
| `internal/cli` | urfave commands, flags/env, human and JSON output |
| `internal/config` | strict YAML, sources, precedence, validation; no CLI dependency |
| `internal/delivery` | routes, message preparation, outcomes, subprocesses |
| `integration/` | built-binary smoke tests (`smoke` build tag) |
| `scripts/` | development checks |
| `backlog/` | the work queue (Backlog.md) |

Add packages only when needed. Do not copy Noon's scheduler, store, HTTP, or
MCP layers. Providers are subprocess adapters, not SDKs or plugins.

## Testing

Implement first, then test critical behavior: parsing, precedence, path and
trust boundaries, no-overwrite writes, and subprocess outcomes. Name the real
failure each new test prevents. Test through public interfaces with real temp
files and local fixture executables, not network mocks.

Tests must never read real user settings or contact real services. Set HOME
and XDG_CONFIG_HOME, and pass config explicitly.

## Backlog and worktrees

Use the Backlog.md CLI in the primary checkout. Claim a task before starting
and reread it after writes. A ready task states scope, non-goals, file
boundaries, dependencies, and executable acceptance. Record command results in
notes; check criteria only after they pass.

Work in the current checkout unless told otherwise. Review `.config/wt.toml`
before creating a Worktrunk worktree; its setup only installs tools, modules,
and hooks.
