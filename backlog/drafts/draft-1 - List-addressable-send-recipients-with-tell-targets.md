---
id: DRAFT-1
title: List addressable send recipients with tell targets
status: Draft
assignee: []
created_date: '2026-10-17 00:19'
updated_date: '2026-10-17 00:19'
labels: []
dependencies: []
priority: medium
type: feature
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: choosing `send.to` means leaving Tell to run `herdr --session S agent list` or `hcom list --json`, reading provider JSON, and guessing which value Tell accepts. Most Herdr agents have no name (only a pane ID), and an agent that is busy or blocked is refused at send time. A read-only `tell targets` puts the recipients Tell would accept, and whether each can receive a message now, in one place. It is also the intended home for future discovery, such as other Herdr sessions or saved machines, behind the same read-only boundary.

Context: delivery never infers a recipient from caller context, and `send.to` stays exactly one explicit recipient (docs/design.md, Agent delivery). Inspection commands never send or start services (AGENTS.md). Tell removes inherited `HERDR_*` variables and requires an explicit `herdr.session`, so a listing behaves the same in Herdr panes and in scheduled services. `ValidRecipient` (internal/delivery/send.go) accepts hcom names and Herdr names or pane IDs. `doctor` (internal/delivery/doctor.go) already runs bounded, sanitized provider probes; reuse its execution and size limits. Provider output such as terminal titles, agent session paths, and process info can contain task or private detail and must not be passed through.

Scope: a `targets` command in internal/cli, listing logic beside the existing send probes in internal/delivery, fixture-executable tests, README and docs/design.md updates.

Non-goals: choosing, ranking, or defaulting a recipient; changing send semantics; delivery to multiple recipients; starting or naming agents; discovery outside the configured route and session (other sessions, `--machine`, network); an MCP tool (decide separately once the CLI shape is settled).

Open question for promotion: list only the configured `send.via`, or every send route whose executable is available.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 `tell targets` lists each recipient for the configured send route that `ValidRecipient` accepts: hcom active names, and Herdr agent names or pane IDs in the explicit `herdr.session`. Each row shows the exact `send.to` value, agent kind, and whether it can receive now (Herdr idle/done vs busy/blocked/unknown).
- [ ] #2 Missing executable, missing or stopped Herdr session, timeout, and malformed or oversized provider output report the same not_sent reasons as `send`/`doctor`. Exit 0 only when the listing completed, including when it finds zero recipients.
- [ ] #3 Fixture-based tests prove `targets` never sends or prompts, never prints terminal titles, session paths, or other raw provider fields, and emits one schema_version 1 object with `--json`.
- [ ] #4 README and docs/design.md describe `targets` as read-only and say that `send` still requires one explicit recipient. `task ci` passes.
<!-- AC:END -->
