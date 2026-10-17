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
Conventional Commit. Never push without instruction. Release publication is wired
to stable version-tag pushes; creating or pushing a tag requires authorization.

## Release packaging

```sh
VERSION=v1.0.0 task cli:package
# Optional reproducible binary metadata overrides:
VERSION=v1.0.0 BUILD_COMMIT="$(git rev-parse HEAD)" \
  BUILD_TIME=2026-10-16T00:00:00Z task cli:package
```

Run from the repository root with a clean source checkout for release builds.
VERSION must be a stable `vX.Y.Z` tag; prereleases are not published by this
workflow. BUILD_COMMIT defaults to HEAD and BUILD_TIME to that commit's timestamp.
The task cross-compiles with CGO disabled and trimpath, injects all three linker
fields, and writes four archives plus `checksums.txt` to `dist/release/vX.Y.Z/`.
Names are `tell-vX.Y.Z-{linux,macos}-{x64,arm64}.tar.gz`; each contains only
`bin/tell`. The task verifies every checksum and archive layout, then extracts
and runs the host binary to compare all version metadata. Run it on Linux or
macOS, x64 or arm64. Generated files remain under ignored `dist/`.

Before authorizing a release, run `task ci`, the packaging task, and
`actionlint .github/workflows/release.yaml`. None publishes anything.
Local checks verify cross-compiled archive layout and host execution, not
execution on all four architectures or live mise asset discovery.

The release workflow adapts hum's exact-commit main-push CI gate and worklease's
archive naming. On a version-tag push it waits for that commit's `ci.yaml`
run to succeed, builds/checks archives, then creates a GitHub Release with
git-cliff release notes, all four archives, checksums, and a full `CHANGELOG.md`
asset generated at the tagged commit. Actions are SHA-pinned;
only the publishing job has contents write permission. Existing releases
are never overwritten. There are no tap updates, changelog commits, or pushes.

### Changelog

`cliff.toml` groups Conventional Commits into Added, Fixed, Performance, Changed,
Documentation, and Reverted sections. Stable `vX.Y.Z` tags delimit releases.
Run `task changelog` to refresh the checked-in `CHANGELOG.md` snapshot; do not
edit generated entries by hand. Commit the snapshot when refreshing it.
The release workflow regenerates the full changelog from Git history, so the
published asset does not depend on that snapshot being current. Each release's
body contains only its own changes (`git-cliff --current --strip header`).
See [GitHub Releases](https://github.com/brettinternet/tell/releases) for the
latest generated changelog and release notes.

After publication, a four-runner matrix installs latest using the plain mise
GitHub backend in fresh cache/config/data/state directories and checks the
expected version. It bypasses mise's shared versions host so newly published
release metadata comes directly from GitHub rather than a stale shared cache. This live check necessarily follows publication and cannot
roll it back. The first release, `v0.1.0`, passed this live check on all four
runners. For subsequent releases, check the release run's matrix results;
local packaging alone does not verify live mise installation.

The [Homebrew tap](https://github.com/brettinternet/homebrew-tap) installs the
same release archives and checksums. Its update workflow checks for new releases
every six hours. Install with `brew install brettinternet/tap/tell` (see the
[installation instructions](../README.md#installation) for initial trust).

## Layout

| Path | Owns |
| --- | --- |
| `cmd/tell` | process boundary, signals, linker-injected build metadata |
| `internal/cli` | urfave commands, flags/env, human and JSON output |
| `internal/config` | strict YAML, sources, precedence, validation; no CLI dependency |
| `internal/delivery` | routes, message preparation, outcomes, subprocesses |
| `internal/mcp` | MCP stdio tools over the shared config and delivery operations |
| `integration/` | built-binary smoke tests (`smoke` build tag) |
| `scripts/` | development checks |
| `backlog/` | the work queue (Backlog.md) |

Tell's MCP adapter uses `github.com/modelcontextprotocol/go-sdk` v1.8.0 for
stdio only. It must call the existing config and delivery operations; do not
copy Noon's scheduler, store, or HTTP layers. The MCP process is owned by its
client and exposes only notify, send, and read-only doctor. Route and recipient
overrides remain default-deny through personal `mcp.agent_targets` configuration.
Current outbound providers are subprocess adapters, not SDKs or plugins. Planned
inbound sources own their network transports and verification; follow the
[planned integration contract](listening.md), without adding listener lifecycle
tools to MCP. See the [README client setup](../README.md#mcp-server) and
[MCP trust boundary](design.md#mcp).

## Testing

Implement first, then test critical behavior: parsing, precedence, path and
trust boundaries, no-overwrite writes, and subprocess outcomes. Name the real
failure each new test prevents. Test through public interfaces with real temp
files and local fixture executables, not network mocks.

Tests must never read real user settings or contact real services. Set HOME
and XDG_CONFIG_HOME, and pass config explicitly. Exercise MCP over the public
protocol with local fixture providers; verify default-deny/opt-in routing,
body-free structured results, and stderr/stdout isolation.

## Backlog and worktrees

Use the Backlog.md CLI in the primary checkout. Claim a task before starting
and reread it after writes. A ready task states scope, non-goals, file
boundaries, dependencies, and executable acceptance. Record command results in
notes; check criteria only after they pass.

Work in the current checkout unless told otherwise. Review `.config/wt.toml`
before creating a Worktrunk worktree; its setup only installs tools, modules,
and hooks.
