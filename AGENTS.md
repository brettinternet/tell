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
- Tell delivers one invocation and exits. Scheduling, queues, durable state, throttling, and deduplication belong to callers such as Noon. Never read scheduler-specific environment or payloads.
- Keep provider credentials/configuration with the provider. Never print message bodies, tokens, or raw provider configuration in diagnostics.

## Backlog
- `backlog/` is the only task queue. Use Backlog.md CLI, never edit task Markdown directly.
- Read `backlog instructions overview` and the relevant creation/execution/finalization guide.
- Claim work in the primary checkout: read the task, mark In Progress, then reread.
- A ready task needs context, scope/non-goals, modified-file boundaries, dependencies, and observable acceptance.
- Record acceptance commands/results before checking criteria. Complete only after verification and commits.
- Do not implement drafts or unrelated tasks.

## Git and isolation
- Work in the current checkout unless instructed otherwise. Preserve unrelated changes.
- When creating a worktree, review .config/wt.toml, then use Worktrunk under .worktrees/.
- Worktree hooks install dependencies/hooks only; no services start.
- Keep Backlog task-state mutations in the primary checkout.
