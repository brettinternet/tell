---
id: TELL-53
title: Route notifications by --level
status: todo
type: feature
priority: medium
labels: [notify]
created: 2026-11-19T16:29:53Z
updated: 2026-11-19T16:29:53Z
---

## Context

Callers can say what happened (`--type`) but not how urgent it is. Today urgency needs provider knowledge at the call site (`--via pushover --pushover-priority emergency`) or a separate config file (`--config critical.yaml`, examples/escalate). Shared scripts and MCP agents should be able to state urgency once, while each person's config decides which routes that means. This keeps the existing split: the caller states intent, personal config owns routing and loudness.

## Scope

- Add `--level low|normal|high|critical` to `notify` and `run` (default `normal`), and a `level` field to the MCP notify tool. No environment variable, matching `--type`.
- Add personal-only `notify.levels` with keys `low`, `high` and `critical`. Each value is a route name or a cascade list in the same form as `notify.via`. `normal` is `notify.via`, so a `normal` key is rejected.
- Resolution: a one-shot via override (`--via`, `TELL_NOTIFY_VIA`, MCP `via`) wins, then `notify.levels[level]`, then `notify.via`. A level's steps merge with top-level provider blocks, and one-shot overrides such as `--apprise-tag` or `--pushover-priority` apply as they do for `notify.via` cascades. `fallback_on` and `fallback_notice` apply unchanged.
- `tell doctor --level L` checks the steps that level would use. `tell config show` displays `notify.levels`.
- Shared `tell.yaml` rejects `notify.levels` like `notify.via`. Update `tell.schema.json` and the shared schema.

## Non-goals

Mapping `--type` to a level or a level to `--type`, user-defined level names (that is `--config`), nearest-level fallback, a `level` on listener route notify actions, time-based escalation or repeats (Noon and the provider own those), and removing `--pushover-priority`.

## Dependencies

None. Builds on the TELL-30 cascade and TELL-42 Pushover route.

## Modified-file boundaries

internal/config (config.go, yaml.go and tests), internal/cli (cli.go, notify, run, doctor and tests), internal/mcp/server.go and its tests, tell.schema.json, tell.shared.schema.json, README.md, docs/configuration.md, docs/design.md.

## Verification

Focused `go test ./internal/cli ./internal/config ./internal/mcp`, direct runs against stderr and command routes, then `task ci`.

## Acceptance criteria

- [ ] notify --level L with notify.levels.L runs exactly that cascade; an unmapped level and no flag use notify.via; a via override beats the level; covered by a CLI test against local routes
- [ ] An invalid --level is a usage error (exit 2) with did-you-mean; notify.levels with an unknown key, a normal key, or in shared tell.yaml is a config error
- [ ] MCP notify accepts level with the same resolution, and tell doctor --level L checks that level's steps
- [ ] README, docs/configuration.md, docs/design.md and both schemas document levels; task ci passes
