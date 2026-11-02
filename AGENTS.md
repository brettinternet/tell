# Agents

Read README.md and docs/development.md before changing Tell. Read AGENTS.local.md if present.

## Development
- Use Go, urfave/cli v3, and the existing strict configuration/CLI patterns.
- Use `task init` for pinned mise tools, Go modules, and Lefthook hooks.
- Implement first, then add tests only for critical behavior or realistic regressions.
- Use focused checks while iterating; run `task ci` before completing substantial work.
- Before every commit, stage intended files and run `task check:staged`.
- Use Conventional Commits. Never push or open a PR without instruction.
- Keep no-argument commands safe: inspect only; never send messages or start services.
- Distinguish implemented features from planned features. No successful stub commands.
- Tell is evolving into a local integrations layer. Outbound notify/send stay one-shot; Slack inspection and explicit --print capture are implemented; planned routes/actions and GitHub follow docs/listening.md and TELL-27 through TELL-29. Explicit source selection starts foreground listeners; bare commands remain inspection. Do not describe planned capabilities as shipped.
- Listeners may own bounded in-memory admission buffers, duplicate suppression, connection recovery, and short local actions. Scheduling, durable queues/state, action retries, and long-running work belong to callers such as Noon. Never read scheduler-specific environment or payloads.
- The desktop route may read `__CFBundleIdentifier` and `HERDR_*` from Tell's own environment to record a notification's origin for click actions; keep stripping `HERDR_*` from child environments.
- Keep outbound provider credentials/configuration with the provider. Inbound sources use personal-only environment-variable references; strip those credentials from child environments. Never print message bodies, tokens, raw provider configuration, or child output in diagnostics. Explicit event export is sensitive data, not diagnostics.

## Backlog
- `backlog/` is this repository's native `bl` task queue. Read and change tasks
  only through `mise exec -- bl` (CLI or MCP), never edit task Markdown directly.
- Read `mise exec -- bl instructions overview` and `mise exec -- bl instructions workflow`
  before acting.
- Pick and claim work in one step, with one stable session for the whole loop:
  `mise exec -- bl next --claim --start --session SESSION --json`. A status
  change is not a claim. Renew with `mise exec -- bl heartbeat --session SESSION`
  before half the TTL.
- A ready task needs context, scope/non-goals, modified-file boundaries, dependencies, and observable acceptance.
- Record each criterion's command and result with
  `mise exec -- bl ac ID check N --evidence TEXT`, not note lines. Complete only
  after verification and commits, then `mise exec -- bl done ID --session SESSION`.
- If stopping incomplete, persist progress then
  `mise exec -- bl release ID --session SESSION`.
- Do not implement drafts or unrelated tasks.

## Git and isolation
- Work in the current checkout unless instructed otherwise. Preserve unrelated changes.
- When creating a worktree, review .config/wt.toml, then use Worktrunk under .worktrees/.
- Worktree hooks install dependencies/hooks only; no services start.
- Keep backlog task-state mutations in the primary checkout.
