# Development

Tooling follows the sibling hum, bump (Noon), and project repositories, trimmed
to a Go-only CLI. Go 1.27.2, urfave/cli v3, and strict YAML configuration match Noon.

## Setup and checks

Run `mise trust && mise exec -- task init` to install pinned tools, Go modules,
and Lefthook hooks. No service starts. The build uses Go; provider runtimes are
optional external tools, not embedded dependencies.

- `task cli:build` builds bin/tell; `task cli:run -- --help` runs from source.
- `task check` checks gofmt, vet, Staticcheck, and changelog generation.
- `task test`, `task race`, and `task smoke` verify source and the built CLI.
- `task security` scans Git history with Gitleaks and reachable Go vulnerabilities.
- `task ci` runs the complete gate. CI runs on Linux and macOS.
- `task check:staged` formats/re-stages Go and scans the staged snapshot.
- `task changelog` generates CHANGELOG.md from Conventional Commits.
- `task backlog` / `task backlog:next` list ready, unclaimed work.

Stage only intended files, run `task check:staged`, and commit with a Conventional
Commit. Never push without instruction. Release publication is not wired up.

## Architecture

- `cmd/tell`: process boundary, signals, linker-injected build metadata.
- `internal/cli`: urfave commands, flag/env handling, human/JSON output.
- `internal/config`: strict YAML, source selection, precedence, validation; no CLI-framework dependency.
- `scripts/`: development checks; no runtime service.
- `backlog/`: authoritative work queue managed through Backlog.md.

Create packages only when functionality needs them. Do not copy Noon's scheduler,
store, HTTP, or MCP layers. Provider integrations are subprocess adapters, not SDKs
or a plugin framework.

## Testing

Implement first. Name the real failure a new test prevents. Cover parsing,
precedence, path/trust boundaries, no-overwrite writes, and subprocess delivery
semantics when implemented. Prefer public-interface tests using real temporary
files; do not test framework behavior or static configuration.

Never read real user settings or contact notification services in tests. Set
HOME/XDG_CONFIG_HOME and pass explicit config inputs; isolate the system config
source too. Provider tests should use local executables, not network mocks.

## Worktrees and backlog

Work in the current checkout unless otherwise directed. Review .config/wt.toml
before Worktrunk creation. Setup only installs tools/modules/hooks.

Read and mutate tasks via Backlog.md CLI in the primary checkout. Claim before
implementing; reread after writes. Each ready task describes scope, non-goals,
modified-file boundaries, dependencies, and executable acceptance. Record command
results in notes and check criteria only after they pass. Future acceptance
commands are targets to implement, not claims of current behavior.
