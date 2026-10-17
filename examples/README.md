# Examples

Real-world recipes for local development, AI agents, and automation. Each
directory has a README and copyable scripts. Scripts need only POSIX `sh` and
Tell unless noted.

Start with a visible personal route, for example `notify.via: desktop` in
`~/.config/tell/config.yaml` ([user-config.yaml](user-config.yaml)), and run
`tell doctor`. Shared settings go in a committed [tell.yaml](tell.yaml).

## Notify yourself

| Example | Scenario | Level | Also uses |
| --- | --- | --- | --- |
| [shell](shell/README.md) | Notify when a slow command finishes; auto-notify for anything over 30s | basic | zsh |
| [taskfile](taskfile/README.md) | Notify from Taskfile `defer` and a Make `%-notify` rule | basic | Task, Make |
| [git-hooks](git-hooks/README.md) | Warn after pull/checkout when lockfiles or migrations changed | basic | Git, Lefthook |
| [wait-for](wait-for/README.md) | Wait for a dev server, PR checks, or a process, then notify | basic | curl, gh |
| [scheduled-checks](scheduled-checks/README.md) | Health, disk, backup, and certificate checks that alert only on change | intermediate | cron, launchd, systemd |
| [noon](noon/README.md) | Noon job hook that notifies on failure and recovery | intermediate | Noon |

## Routes and integrations

| Example | Scenario | Level | Also uses |
| --- | --- | --- | --- |
| [command-routes](command-routes/README.md) | Your own provider: phone via ntfy, quiet hours, audit log, tmux, speech | intermediate | curl, ntfy |
| [apprise](apprise/README.md) | Phone, Slack, and Discord with per-message audience tags | intermediate | Apprise |
| [github-actions](github-actions/README.md) | Post failed nightly runs to a team channel | intermediate | GitHub Actions, Apprise |

## AI agents

| Example | Scenario | Level | Also uses |
| --- | --- | --- | --- |
| [claude-code](claude-code/README.md) | Notify on permission prompts and long finished turns | basic | Claude Code hooks, jq |
| [codex](codex/README.md) | Notify when a Codex turn completes | basic | Codex, jq |
| [mcp](mcp/README.md) | Let agents notify you or message other agents; opt-in route choice | intermediate | any MCP client |
| [local-llm-summary](local-llm-summary/README.md) | Put a local model's one-line diagnosis in failure notifications | intermediate | Ollama, bash |

## Agent cross-communication

| Example | Scenario | Level | Also uses |
| --- | --- | --- | --- |
| [agent-handoff](agent-handoff/README.md) | Implementer hands a branch to a reviewer agent; planner → implementer | intermediate | hcom, Herdr, Git |
| [test-watch-agent](test-watch-agent/README.md) | Tell the coding agent when its edits break or fix the tests | advanced | watchexec, hcom |
| [ci-to-agent](ci-to-agent/README.md) | Hand failed PR checks to a fixer agent; tell you whether it was reached | advanced | gh, jq, hcom |
| [pr-review-queue](pr-review-queue/README.md) | First-pass reviews of newly requested PRs, sent once each | advanced | gh, Noon |
| [eval-regression](eval-regression/README.md) | Catch prompt/model quality drops; ask an agent to find the cause | advanced | jq, any eval harness |
| [fallback](fallback/README.md) | Try a Herdr pane, then hcom, then a person, only when safe | advanced | Herdr, hcom, jq |

## Patterns used throughout

| Pattern | Why | See |
| --- | --- | --- |
| Send pointers, not content | Logs, PR text, and transcripts are untrusted and may hold secrets; recipients fetch with their own tools | [ci-to-agent](ci-to-agent/README.md) |
| Alert on transitions | Repeated alerts get ignored; the caller keeps the state | [scheduled-checks](scheduled-checks/README.md) |
| Branch on the exit code | 3 means nothing was sent (retry or fall back); 4 and 5 may have arrived (check first) | [fallback](fallback/README.md) |
| `\|\| true` in hooks | A delivery problem must not fail the build, hook, or agent | [taskfile](taskfile/README.md) |
| Literal messages, `</dev/null` | Hook stdin (JSON payloads) never becomes a message | [claude-code](claude-code/README.md) |
| Routing stays personal | Shared files say what happened; your config says where it goes | [apprise](apprise/README.md) |
