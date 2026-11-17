---
id: TELL-46
title: Test listener routes offline from captured events
status: todo
type: feature
priority: medium
labels: [listen]
created: 2026-11-17T19:51:37Z
updated: 2026-11-17T19:51:37Z
---

## Context

Checking a route's `match` predicate or template today needs a live event: create a Slack channel, react to a message, or push a webhook. `tell listen --print` already exports envelopes as NDJSON, but nothing reads them back, so authoring routes is slow trial and error against real services.

## Scope

- `tell listen --dry-run --from FILE` (`-` for stdin) reads NDJSON envelopes in `--print` format and runs each through the same admission (duplicate suppression), matching, rendering, and preparation as a live dry-run. Receipts on stderr are the existing dry-run receipts.
- Opens no connection, binds no port, reads no source credentials, resolves no executables, runs no actions, and probes no destinations. Routes load and validate as usual, but source credentials need not be set.
- `--from` requires `--dry-run`. Combined with source names, `--all`, or `--print`, it is a usage error (exit 2). Envelopes with no matching route get the existing no-match receipt.
- Each line is validated like a live envelope (schema_version 1, metadata rules, 256 KiB cap). A malformed line gets a body-free receipt with its line number, and processing continues. Exit 1 if the file is unreadable or any line was malformed, else 0.
- Document the authoring loop (capture once with `--print`, iterate with `--from`) in docs/listening.md and examples/listen.

## Non-goals

Running actions from a file (that is replay; decision-1), showing rendered text, recording or storing events, synthesizing events, mixing live sources with a file, and watching the file for changes.

## Dependencies

None. Related: TELL-26, TELL-27, TELL-28, TELL-29, decision-1.

## Modified-file boundaries

internal/cli/listen.go, internal/listen (file ingress through the existing admission and dry-run path), existing tests in internal/cli and internal/listen, docs/listening.md, README.md (Slack command list), examples/listen/README.md.

## Verification

Fixture envelopes through the built binary with credentials unset and no network; `go test -race ./internal/listen ./internal/cli`; `task ci`.

## Acceptance criteria

- [ ] tell listen --dry-run --from FILE reports the same per-route match, no-match, and duplicate receipts as a live dry-run of the same envelopes; covered by a test with fixture envelopes
- [ ] With source credentials unset and no network it succeeds, and no connection, bind, executable resolution, or action start occurs; covered by a test
- [ ] Malformed lines get body-free receipts with line numbers, valid lines still process, and the exit is 1; no receipt contains envelope data, rendered text, or argv (sentinel test)
- [ ] --from without --dry-run, or with source names, --all, or --print, exits 2
- [ ] docs/listening.md, README.md, and examples/listen/README.md document the capture-then-test loop; task ci passes
