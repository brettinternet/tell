---
id: TELL-46
title: Test listener routes offline from captured events
status: done
type: feature
priority: medium
labels: [listen]
created: 2026-11-17T19:51:37Z
updated: 2026-11-17T20:17:37Z
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

- [x] tell listen --dry-run --from FILE reports the same per-route match, no-match, and duplicate receipts as a live dry-run of the same envelopes; covered by a test with fixture envelopes
  Evidence: e32bb2c: TestListenDryRunEvents (internal/cli) runs fixture envelopes through real listen.Capture via --dry-run --events; asserts exact accepted/duplicate/route dry_run/no_match receipts, same format code as live runSources. Binary smoke: 5000-line file -> 15000 receipts exit 0; GitHub digest duplicate verified.
- [x] With source credentials unset and no network it succeeds, and no connection, bind, executable resolution, or action start occurs; covered by a test
  Evidence: TestListenDryRunEvents sets TELL_TEST_SLACK_SECRET empty, route run script marker never created; file path builds only fileSpec (no slack/github spec, no bind). Binary run with env -u credential succeeded offline.
- [x] Malformed lines get body-free receipts with line numbers, valid lines still process, and the exit is 1; no receipt contains envelope data, rendered text, or argv (sentinel test)
  Evidence: TestListenDryRunEvents: invalid_json line=3, invalid_envelope line=6, event_size line=7 receipts; later lines processed; exit 1; stderr contains no 'sentinel' data, argv path, or rendered 'Alert' text.
- [x] --from without --dry-run, or with source names, --all, or --print, exits 2
  Evidence: TestListenCLISelection table: listen --events -, --dry-run --events - slack, --dry-run --events - --all, --print --events - all exit 2; binary also exit 2 with --json.
- [x] docs/listening.md, README.md, and examples/listen/README.md document the capture-then-test loop; task ci passes
  Evidence: docs/listening.md 'Testing routes offline' + CLI/exit tables, README.md command list, examples/listen/README.md capture-then-test loop, docs/configuration.md flag list. task ci exit 0 after rebase onto 23fbcfa (go test, -race, staticcheck, govulncheck, smoke).

## Notes

Worktree: .worktrees/tell-46-listen-from branch tell-46-listen-from (session 5ce4f1e5-a034-4c18-af9e-55883dfaf12e). Plan: implement --from file ingress through existing dry-run path, merge to main.

Flag renamed --from -> --events (user decision 2026-11-17): global --from already sets send.from and listen inherits it for send actions. Criteria below refer to --from; --events is the implemented flag. Merged to main as e32bb2c (ff from tell-46-listen-from). Self-review fix: GitHub file envelopes also dedupe by body digest like live admission.
