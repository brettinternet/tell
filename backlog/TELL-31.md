---
id: TELL-31
title: List addressable send recipients with tell targets
status: done
type: feature
priority: medium
rank: ae969696969696959
created: 2026-10-17T00:19:00Z
updated: 2026-11-02T04:20:04Z
---

Why: choosing `send.to` means leaving Tell to run `herdr --session S agent list` or `hcom list --json`, reading provider JSON, and guessing which value Tell accepts. Most Herdr agents have no name (only a pane ID), and an agent that is busy or blocked is refused at send time. A read-only `tell targets` puts the recipients Tell would accept, and whether each can receive a message now, in one place. It is also the intended home for future discovery, such as other Herdr sessions or saved machines, behind the same read-only boundary.

Context: delivery never infers a recipient from caller context, and `send.to` stays exactly one explicit recipient (docs/design.md, Agent delivery). Inspection commands never send or start services (AGENTS.md). Tell removes inherited `HERDR_*` variables and requires an explicit `herdr.session`, so a listing behaves the same in Herdr panes and in scheduled services. `ValidRecipient` (internal/delivery/send.go) accepts hcom names and Herdr names or pane IDs. `doctor` (internal/delivery/doctor.go) already runs bounded, sanitized provider probes; reuse its execution and size limits. Provider output such as terminal titles, agent session paths, and process info can contain task or private detail and must not be passed through.

Scope: a `targets` command in internal/cli, listing logic beside the existing send probes in internal/delivery, fixture-executable tests, README and docs/design.md updates.

Non-goals: choosing, ranking, or defaulting a recipient; changing send semantics; delivery to multiple recipients; starting or naming agents; discovery outside the configured route and session (other sessions, `--machine`, network); an MCP tool (decide separately once the CLI shape is settled).

Decision: list only the configured `send.via` route, matching how `doctor` probes. Listing other available routes is a non-goal for now.

## Acceptance criteria

- [x] `tell targets` lists each recipient for the configured send route that `ValidRecipient` accepts: hcom active names, and Herdr agent names or pane IDs in the explicit `herdr.session`. Each row shows the exact `send.to` value, agent kind, and whether it can receive now (Herdr idle/done vs busy/blocked/unknown).
  Evidence: Commit 08eeee3. internal/delivery/targets.go lists hcom active/listening/blocked names and Herdr names or pane IDs (ValidRecipient) with to/kind/status/ready. Verified live: tell targets --send-via herdr --herdr-session default listed 8 agents, idle=ready, working=not ready. TestTargetsReadOnly asserts exact rows.
- [x] Missing executable, missing or stopped Herdr session, timeout, and malformed or oversized provider output report the same not_sent reasons as `send`/`doctor`. Exit 0 only when the listing completed, including when it finds zero recipients.
  Evidence: TestTargetsReadOnly failure cases pass: missing_executable (hcom, herdr), session_not_running (doctor's session probe), timeout, invalid_response for malformed and >1MiB output, all not_sent exit 3; zero recipients exits 0. Live: unknown session -> not_sent/session_not_running exit 3.
- [x] Fixture-based tests prove `targets` never sends or prompts, never prints terminal titles, session paths, or other raw provider fields, and emits one schema_version 1 object with `--json`.
  Evidence: go test ./internal/cli -run Targets: fixture records .sent on any non-probe call; asserted absent. Fixture includes titles, cwd, agent_session paths, odd kinds; asserted absent from JSON and human output. --json decodes to exactly one schema_version 1 object.
- [x] README and docs/design.md describe `targets` as read-only and say that `send` still requires one explicit recipient. `task ci` passes.
  Evidence: README Agent messages, docs/design.md Commands + Recipient listing, docs/configuration.md updated: targets read-only, send still requires one explicit recipient. task ci passed (check, test, race, smoke, security).

## Summary

Added read-only tell targets (internal/delivery/targets.go, internal/cli/targets.go) listing configured send-route recipients with exact send.to value, kind, status, ready. Shares readOnlyFailure with send probes and Herdr session check with doctor. Merged to main as 08eeee3; task ci passed. Self-review: no item-scoped defects. Note: Herdr agent 'name' field read from snapshot struct (verified in herdr 0.9.5 binary), not yet exercised against a renamed live agent.
