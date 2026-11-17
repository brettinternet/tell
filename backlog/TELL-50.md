---
id: TELL-50
title: Write desktop routing in tell init --user when desktop is ready
status: todo
type: feature
priority: low
labels: [config]
created: 2026-11-17T19:51:46Z
updated: 2026-11-17T19:51:46Z
---

## Context

After `tell init --user`, `notify.via` is commented out, so the first `tell notify hi` prints to the same terminal (the stderr default) and looks like nothing happened. Most people want desktop banners.

## Scope

- `tell init --user` checks desktop readiness exactly as `tell doctor --via desktop` does (no send). When ready, it writes `notify.via: desktop` uncommented and the next-step line says `tell notify hi` will show a banner. Presence (a locked screen) does not count against readiness.
- When not ready, the file matches today's template and the next step names the missing piece using doctor's remediation text.
- `--local`, shared `tell init`, `--json` output shape, and no-overwrite behavior are unchanged.

## Non-goals

Probing other routes (iMessage may open Messages; Apprise and Pushover need credentials), interactive prompts, sending a test notification, and changing the stderr default when no config exists.

## Dependencies

None.

## Modified-file boundaries

internal/cli/cli.go (initialize), internal/config/init.go, internal/config/template.go, existing init tests, README.md (Quick tour, Configure), docs/configuration.md (init).

## Verification

Built-binary `tell init --user` in an isolated HOME on macOS, then `tell config check` and `tell doctor`; `task ci`.

## Acceptance criteria

- [ ] With desktop ready, tell init --user writes notify.via: desktop and tell config check passes on the result; when not ready the file matches the current template and the next step names the missing piece; covered by a test
- [ ] init sends nothing and runs no provider beyond the desktop readiness probe doctor already uses; --local, shared init, --json shape, and no-overwrite behavior are unchanged
- [ ] README.md and docs/configuration.md describe the new first-run behavior; task ci passes
