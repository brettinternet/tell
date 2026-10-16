# Development

Tooling follows the sibling hum, bump (Noon), and project repositories, trimmed
to a Go-only CLI. Go 1.27.2, urfave/cli v3, and strict YAML configuration match Noon.

## Setup and checks

Run `mise trust && mise exec -- task init` to install pinned tools, Go modules,
and Lefthook hooks. No service starts. The build uses Go; provider runtimes are
optional external tools, not embedded dependencies. After configuring a delivery
route, run `tell doctor` to check readiness without sending; see
[route readiness](configuration.md#route-readiness) for its limits.

- `task cli:build` builds bin/tell; `task cli:run -- --help` runs from source.
- `task check` checks gofmt, vet, Staticcheck, and changelog generation.
- `task test`, `task race`, and `task smoke` verify source and the built CLI.
- `task security` scans Git history with Gitleaks and reachable Go vulnerabilities.
- `task ci` runs the complete gate. CI runs on Linux and macOS.
- `task check:staged` formats/re-stages Go and scans the staged snapshot.
- `task changelog` generates CHANGELOG.md from Conventional Commits.
- `task backlog` / `task backlog:next` list ready, unclaimed work.

Stage only intended files, run `task check:staged`, and commit with a Conventional
Commit. Never push without instruction. Release publication is wired to stable
version-tag pushes; creating or pushing a tag requires explicit authorization.

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
generated notes, all four archives and checksums. Actions are SHA-pinned;
only the publishing job has contents write permission. Existing releases
are never overwritten. There are no tap updates, changelog commits, or pushes.

After publication, a four-runner matrix installs latest using the plain mise
GitHub backend in fresh cache/config/data/state directories and checks the
expected version. This live check necessarily follows publication and cannot
roll it back. Until a release is authorized and published, record live mise
installation as **pending**, not passed. First-release publication is not
part of local implementation verification.

## Architecture

- `cmd/tell`: process boundary, signals, linker-injected build metadata.
- `internal/cli`: urfave commands, flag/env handling, human/JSON output.
- `internal/config`: strict YAML, source selection, precedence, validation; no CLI-framework dependency.
- `scripts/`: development checks; no runtime service.
- `integration/`: built-binary smoke tests behind the `smoke` build tag.
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
HOME/XDG_CONFIG_HOME and pass explicit config inputs. There is no system config
source. Provider tests should use local executables, not network mocks.

## Worktrees and backlog

Work in the current checkout unless otherwise directed. Review .config/wt.toml
before Worktrunk creation. Setup only installs tools/modules/hooks.

Read and mutate tasks via Backlog.md CLI in the primary checkout. Claim before
implementing; reread after writes. Each ready task describes scope, non-goals,
modified-file boundaries, dependencies, and executable acceptance. Record command
results in notes and check criteria only after they pass. Future acceptance
commands are targets to implement, not claims of current behavior.
