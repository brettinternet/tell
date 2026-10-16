---
id: TELL-14
title: 'Serve notify, send, and doctor as MCP tools over stdio'
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 22:57'
updated_date: '2026-10-16 23:49'
labels:
  - reviewed
dependencies: []
references:
  - 'https://github.com/modelcontextprotocol/go-sdk'
documentation:
  - docs/design.md
  - docs/development.md
  - ../bump/internal/mcp/server.go
  - ../bump/internal/mcp/stdio.go
  - ../bump/internal/cli/mcp.go
priority: high
type: feature
ordinal: 6500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: agents (Claude Code, Codex, pi) should be able to notify a person or message another agent without shelling out to `tell notify`/`tell send` and parsing exit codes. A stdio MCP server started by the agent client (`tell mcp`) exposes the same delivery semantics as typed tools. It is not a daemon: the client owns the process, and each tool call is still exactly one delivery with one outcome and no retry.

This reverses an explicit non-goal. docs/design.md ("No ... ACP/MCP server ... in the initial delivery implementation") and docs/development.md ("Do not copy Noon's ... MCP layers") must be updated to describe the MCP boundary instead.

Routing trust boundary (user decision): agents cannot choose where messages go unless the user allows it. Setting `mcp.agent_targets` (boolean, default false; flag `--mcp-agent-targets`, env `TELL_MCP_AGENT_TARGETS`) controls this. It is personal-only like other routing: accepted in user config, private .tell.yaml, explicit --config, env, and flags; rejected in shared tell.yaml so a repository cannot grant agents routing control. When false, tools take only content fields (message, title, type, intent) and routes/targets come from the same layered configuration as the CLI, including flags/env passed to `tell mcp`. A user who enabled it globally can force it off for one client or project with `tell mcp --mcp-agent-targets=false` or `.tell.yaml`. When true, agents may only choose route and target selectors (notify route, Herdr session, Apprise tag, send route, send recipient), never executable paths (notify.command), provider config files (apprise.config), sender identity (send.from), timeout, or secrets policy. Existing safety still applies: single recipient, no broadcast/fan-out, provenance header, normalization, redaction/reject policy, readiness probes.

Prior art: Noon (../bump) `noon mcp` uses github.com/modelcontextprotocol/go-sdk (v1.8.0) with internal/mcp/server.go, internal/mcp/stdio.go, and internal/cli/mcp.go (terminal stdin prints client setup instead of serving; --json rejected because stdout is the protocol). Reuse that shape; do not copy Noon's scheduler, store, or HTTP layers.

Scope: `tell mcp` command, internal/mcp package (new), notify/send/doctor tools reusing internal/delivery, the personal opt-in setting in internal/config, docs (README, design, configuration, development), tests.

Non-goals: HTTP/SSE transports; resources or prompts; MCP-specific retries, queues, or waiting for agent replies; spawning agents; exposing config editing or init; returning message bodies or raw provider output in tool results.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 `tell mcp` serves MCP over stdio exposing exactly three tools: notify (message, optional title and type), send (message, optional intent), and doctor (read-only, annotated readOnlyHint); with a terminal on stdin it prints client setup instructions and serves nothing, and `--json` is a usage error.
- [x] #2 Each notify/send tool call performs at most one delivery and returns structured content with schema_version, route, outcome, redactions, and on failure the same error code/reason as CLI --json; send also returns recipient and intent; non-accepted outcomes set isError; results never contain message bodies or raw provider output.
- [x] #3 Configuration is loaded per tool call from the same layered sources as the CLI (project/config selection and setting flags/env given to `tell mcp` included), so edits apply without restarting; invalid config yields a config error result and the server keeps serving.
- [x] #4 The stderr notify route writes only to the server's stderr, never to the protocol stream on stdout; timeout or client cancellation after provider start reports uncertain.
- [x] #5 task test exercises the server through the public MCP protocol with local fixture executables for at least one notify route and both send routes, covering default-deny overrides, opt-in overrides, and outcome mapping; docs (README, design, configuration, development) describe the MCP boundary, opt-in trust tradeoff, and client setup; task ci passes.
- [x] #6 With `mcp.agent_targets` false (the default), tool schemas expose no route or target fields and a call supplying them is rejected without delivery. With it true, notify and send accept route/target overrides limited to notify route, Herdr session, Apprise tag, send route, and send recipient, and never executable paths, provider config files, sender identity, timeout, or secrets policy. `tell config` shows its value and origin; setting it in shared tell.yaml is a config error; `tell mcp --mcp-agent-targets=false` overrides an enabled user config.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Reuse Noon stdio SDK shape and Tell delivery/config patterns; add personal-only mcp.agent_targets and per-call config loading. 2. Implement typed notify/send/doctor tools with strict routing boundaries and sanitized CLI-equivalent results. 3. Exercise public protocol, update four docs, run task ci and independent boundary review. 4. Commit implementation, merge to main, finalize task evidence and remove owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implementation complete in owned .worktrees/tell-14-mcp; initial task ci and focused protocol/config checks passed. Single independent review reproduced stale schemas after live opt-in changes and SDK malformed-parameter body disclosure. Live-schema correction and bidirectional regression pass task ci. Confidentiality correction is underway. Binary cancellation after fixture provider start returned uncertain/cancelled; no user decision needed. Next: verify confidentiality regression, commit, merge and cleanup.

Verified all criteria: mise exec -- task ci passed after both review corrections and again on integrated main (unit, race, smoke, vet, Staticcheck, formatting, security). Public MCP tests cover exactly three tools, read-only doctor, configured notify/Apprise and hcom/Herdr send, allowlisted/default-deny overrides, per-call config edits/recovery, outcomes and confidentiality. Bidirectional live opt-in schema regression passes. Manual binary checks: terminal stdin prints client setup and exits; protocol notify keeps body on stderr; cancellation after provider start returns uncertain/cancelled; malformed metadata returns generic -32602 without body. Config/CLI tests verify strict personal-only bool, origin and false flag precedence. Single independent review found stale schemas and SDK error disclosure; both fixed with regression evidence. Implementation commit 23d51bd (rebased from 7dffe46) merged locally to main; no push. Owned worktree/branch tell-14-mcp removed via Worktrunk; Herdr wEA verified absent. Unrelated review-tell-4-5-6 left untouched. No blockers or remaining implementation steps.

Review: no defects found; no code changes. Traced per-call config loading, two-stage override gate (fields allowlisted from live config, rechecked after applying overrides), live tools/list schemas, response sanitization and EOF drain in ServeStdio. Built-binary probes over stdio: notify accepted with body only on server stderr; default-deny via override returns usage/agent_targets_disabled without delivery; non-object arguments, unknown tool and a notification method sent with an id return generic errors without echoing the body; client responses with unknown ids do not stall shutdown. Agent-supplied herdr_session cannot inject Herdr options: Herdr 0.9.5 session.rs consumes the next argv unconditionally and validates the name before remote/machine parsing. task ci passed.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Added tell mcp with notify/send/doctor, live layered configuration, personal-only routing opt-in and sanitized structured results. Public protocol regressions and integrated task ci pass. Committed and merged as 23d51bd; owned worktree and workspace cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->
