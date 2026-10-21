# Development

Go 1.27.3, urfave/cli v3, and strict YAML, matching Noon.

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
| `task smoke` | test the built binary in an isolated environment |
| `task security` | Gitleaks over history, govulncheck |
| `task ci` | all of the above; CI runs it on Linux and macOS |
| `task check:staged` | format and re-stage Go, scan the staged snapshot |
| `task changelog` | write CHANGELOG.md from Conventional Commits |
| `task backlog` / `task backlog:next` | ready, unclaimed work |

Before committing: stage only intended files, run `task check:staged`, and use
a Conventional Commit. Never push or tag without instruction; pushing a stable
version tag publishes a release.

## Release packaging

```sh
VERSION=v1.0.0 task cli:package
# reproducible metadata overrides (defaults: HEAD and its commit time)
VERSION=v1.0.0 BUILD_COMMIT="$(git rev-parse HEAD)" BUILD_TIME=2026-10-16T00:00:00Z task cli:package
```

```text
dist/release/v1.0.0/
├── checksums.txt
├── tell-v1.0.0-linux-arm64.tar.gz    # each contains only bin/tell
├── tell-v1.0.0-linux-x64.tar.gz
├── tell-v1.0.0-macos-arm64.tar.gz
└── tell-v1.0.0-macos-x64.tar.gz
```

Run from a clean checkout on Linux or macOS. `VERSION` must be a stable
`vX.Y.Z` (no prereleases). The task cross-compiles with CGO off and
`-trimpath`, verifies checksums and layout, and runs the host binary to check
its version metadata. It does not run the other architectures.

Before authorizing a release, run `task ci`, `task cli:package`, and
`actionlint .github/workflows/release.yaml`. None publishes.

On a version-tag push, the release workflow:

1. waits for that commit's `ci.yaml` run to pass;
2. builds and checks the archives;
3. creates a GitHub Release with git-cliff notes, archives, checksums, and a full `CHANGELOG.md`, never overwriting an existing release;
4. installs latest with mise's GitHub backend on all four platforms, in fresh directories, and checks the version.

Step 4 runs after publication and cannot roll it back; check its matrix results.
Actions are SHA-pinned and only the publishing job can write contents. The
[Homebrew tap](https://github.com/brettinternet/homebrew-tap) picks up new
releases every six hours.

### Changelog

`cliff.toml` groups Conventional Commits into Added, Fixed, Performance,
Changed, Documentation, and Reverted; stable tags delimit releases. Run
`task changelog` to refresh and commit `CHANGELOG.md`; never hand-edit entries.
Releases regenerate it from history, and each release body holds only its own
changes (`git-cliff --current --strip header`).

## Layout

| Path | Owns |
| --- | --- |
| `cmd/tell` | process boundary, signals, linker-injected build metadata |
| `internal/cli` | urfave commands, flags/env, human and JSON output |
| `internal/config` | strict YAML, sources, precedence, validation; no CLI dependency |
| `internal/delivery` | routes, message preparation, outcomes, subprocesses |
| `internal/listen` | Slack and GitHub sources, admission/dedup, actions, output, recovery |
| `internal/mcp` | MCP stdio tools over the shared config and delivery operations |
| `integration/` | built-binary smoke tests (`smoke` build tag) |
| `scripts/` | development checks |
| `backlog/` | the work queue (Backlog.md) |

Providers are plain subprocess adapters, not SDKs, plugins, or an interface
registry. MCP uses `github.com/modelcontextprotocol/go-sdk` v1.8.0 over stdio
and must call existing config and delivery operations; do not copy Noon's
scheduler, store, or HTTP layers, or add listener tools to MCP. Slack uses
coder/websocket with an internal fixture seam, never a public API URL option.
Follow the [listener contract](listening.md) for inbound work.

## Testing

Implement first, then test critical behavior: parsing, precedence, trust
boundaries, no-overwrite writes, and subprocess outcomes. Name the real failure
each test prevents. Use public interfaces, real temp files, and local fixture
executables, not network mocks.

Tests never read real user settings or contact real services: set HOME and
XDG_CONFIG_HOME and pass config explicitly.

| Area | Check |
| --- | --- |
| MCP | public protocol with fixture providers; default-deny/opt-in routing, body-free results, stdout/stderr isolation |
| listeners | `go test -race ./internal/listen ./internal/config ./internal/cli ./internal/delivery`: local HTTP/WebSocket auth and acks, malformed input, bounds, recovery, selectors, credential stripping |
| signals | `task smoke` sends real SIGINT/SIGTERM against a rejecting localhost proxy |

Print mode is sensitive export, so fixture diagnostics must never contain
body, token, or ticket sentinels.

## Backlog and worktrees

Use the Backlog.md CLI in the primary checkout. Claim a task before starting
and reread it after writes. A ready task states scope, non-goals, file
boundaries, dependencies, and executable acceptance. Record command results
before checking criteria.

Work in the current checkout unless told otherwise. Review `.config/wt.toml`
before creating a Worktrunk worktree; its setup installs tools, modules, and
hooks only.
