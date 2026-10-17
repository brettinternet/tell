---
id: decision-1
title: >-
  Tell is the local integration edge in both directions; schedulers own
  background work
date: '2026-10-17 20:59'
status: accepted
---
## Context

Tell started outbound-only: one invocation, one delivery, then exit, with no daemon,
state, queue, or retries. Event-driven automation on a developer machine (a Slack incident
channel is created, so start local work) also needs an inbound edge: hold a provider
connection, filter events, and hand them off. Options considered:

- Noon built-in sources. Noon refuses long-running listeners, provider credentials, and
  provider catalogs; its scheduler should only execute.
- A separate inbound tool. Clean boundary, but another binary, config, and service for the
  same concerns Tell already owns: provider integration, personal-only routing trust, safe
  argv, and bounded subprocesses.
- Tell. One integration layer in both directions, if it stays free of background work.

The target is a developer machine during working hours, not a durable cloud service.

## Decision

1. Tell owns the local integration edge in both directions: outbound `notify` and `send`,
   inbound `tell listen`.
2. `tell listen` is a foreground process. It never self-daemonizes or installs a service;
   run it in a terminal or under hum, launchd, or systemd. No other command starts it, and
   inspection (`tell`, `config`, `doctor`) never connects to a provider.
3. Tell keeps only in-memory state: connections, a bounded deduplication window, and
   in-flight actions. No database, durable queue, retries, run history, or scheduling.
   Events sent while Tell is not connected may be lost, and docs say so.
4. Route actions hand off and return quickly: an argv command with the event on stdin and
   a bounded timeout. Work that needs durability, retries, history, or a long runtime goes
   to a scheduler, for example `noon run JOB --detach --key ID --data -`. Tell never runs
   background jobs.
5. Sources, routes, and credentials are personal configuration only (user config,
   `.git/tell.yaml`, explicit `--config`). Shared `tell.yaml` cannot define them.
   Credentials come from environment variables named in config, never YAML values; Tell
   never logs them and removes them from every child environment.
6. Event data never becomes argv, shell text, or executable selection.
7. Each source is built in (no plugin registry), owns its transport and verification, and
   emits one common envelope. Adding a source does not change route or action contracts.

## Consequences

- "No daemon" narrows to outbound commands: `tell listen` is the one long-running command.
  README, design, and AGENTS.md change when the first listener ships.
- Each new source needs a task that settles its authentication, event mapping, and
  deduplication identity.
- Missed events while disconnected or asleep are accepted. Workflows that must not miss an
  event reconcile on a schedule in the scheduler, not in Tell.
- If listeners seem to need durability, the answer is handing off to a scheduler, not
  adding state to Tell.

