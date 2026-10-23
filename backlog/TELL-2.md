---
id: TELL-2
title: Deliver human notifications through stderr and Apprise
status: done
type: feature
assignees: ['@pi']
labels: [reviewed]
blocked_by: [TELL-1, TELL-7]
rank: a1696969696969695
refs: ['https://github.com/caronc/apprise/blob/master/apprise/cli.py', docs/design.md, docs/configuration.md, ../bump/docs/hooks.md]
created: 2026-10-16T20:28:00Z
updated: 2026-10-16T23:49:00Z
---

Why: people and Noon jobs need one stable notify command instead of duplicating provider argv in scripts. This task defines the notify contract (message input, title, notification type, JSON result, outcomes) and the bounded subprocess helper that the desktop, command, Herdr, hcom, and Noon hook tasks reuse.

Context not recoverable from the code:
- The outcome contract (exit codes 0-5: accepted, config, usage, not_sent, failed, uncertain) and JSON result shape are defined in docs/design.md. Every route maps into it; unknown provider exits map to failed.
- The built-in default route writes to stderr, not stdout: a calling script's stdout may be captured as data, and --json needs stdout for exactly one object. The bootstrap config value notify.via: stdout becomes stderr.
- Noon hooks receive a JSON run summary on stdin and append hook stdout/stderr to the run log (../bump/docs/hooks.md). Tell must never consume stdin unless the caller explicitly selects it.
- Apprise CLI: --title/-t, --body/-b (reads stdin when omitted), --config/-c, --tag/-g, --input-format/-i text|markdown|html, --notification-type/-n info|success|warning|failure. Without --tag only untagged services are notified. With no --config and no URLs, Apprise uses APPRISE_CONFIG_PATH/default config paths or APPRISE_URLS. Positional URLs override --config and --tag. Statuses include partial delivery, no matching service/tag, and timeout; confirm their numeric exit codes before mapping them.
- Bootstrap text says nothing is sent: README implemented/not-implemented lines, the CLI root description, the init template header, the trailer of tell config output, docs/design.md, docs/configuration.md, and the smoke assertion that notify fails.

Scope: notify command in internal/cli; new internal/delivery package (stderr, Apprise, shared subprocess helper); internal/config for the stderr rename and options this task exercises; CLI tests with local fixture executables; integration/smoke_test.go; README, docs, examples, and the init template.

Non-goals: desktop, command, Herdr, or hcom routes; message normalization and secret redaction (separate tasks); scheduling or queues; automatic retries; fallback to another route; provider SDKs; shell templates; plugin registry; Apprise URLs or credentials in Tell config.

## Acceptance criteria

- [x] task test and task smoke exercise tell notify with a literal message and with stdin selected explicitly (for example -); without explicit selection notify never reads stdin, an empty message fails before any provider runs, and the default stderr route writes the message to stderr without launching a provider, leaving stdout untouched.
- [x] CLI tests use a real local fixture executable to verify exact Apprise argv for title, body (always passed with --body), config, tag, format, and notification type (info, success, warning, failure; default info); unset Tell options are omitted so native APPRISE_* behavior applies. No real service is contacted.
- [x] Results follow the outcome table in docs/design.md: tests cover a missing executable (3 not_sent), provider failure and partial delivery (4 failed), and timeout and cancellation after start (5 uncertain). Provider subprocesses never inherit Tell's stdin, and diagnostics never echo message bodies, credential URLs, or raw provider output.
- [x] tell --json notify emits exactly one schema_version 1 object with outcome and route, plus error code, message, and reason on failure, for every route; it never contains the message body.
- [x] Without --title, the title defaults to the selected project's directory name; notify.via accepts stderr instead of stdout in config, docs, examples, and the init template.
- [x] README, docs, CLI help, the init template, and tell config output describe notify as implemented, state that provider acceptance is not human acknowledgment, and list Apprise as an optional external dependency; task ci passes.

## Definition of done

- [x] Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.

## Plan

1. Confirm Apprise exit semantics; implement concrete delivery outcomes and bounded subprocess execution with private diagnostics. 2. Add notify input/title/type handling and machine results; change default routing to stderr. 3. Exercise real local provider fixtures, input isolation, cancellation, outcomes, and binary smoke behavior. Update user documentation and templates. 4. Run task ci and one independent defect review, commit implementation, merge main, record completion, and remove owned worktree.

## Notes

Implemented in .worktrees/tell-2-notify (branch tell-2-notify). Confirmed Apprise result enum and CLI against upstream 8227752a16c630536daefc387a0e546580fed5ae: 0 success, 3 no_match, 4 partial, 5 timeout; other exits fail. task ci passed (unit tests, race, vet, Staticcheck, smoke, Gitleaks, govulncheck, changelog validation). Local fixtures verify exact argv, stdin isolation, title/type defaults, failure/partial/missing executable and timeout/cancellation; no real services contacted. Documentation/templates updated. Single independent delivery-safety review in progress; then commit, merge, and owned-worktree cleanup.

Final acceptance: task ci passed after review fixes, including test, race, smoke, vet, Staticcheck, changelog, Gitleaks and govulncheck. AC1: TestNotifyInputIsolation and built-binary smoke cover literal/explicit stdin, untouched stdout, empty-input rejection before providers. AC2/5: TestAppriseArguments verifies real fixture argv for all types, default project title, optional arguments omitted; config tests verify stderr defaults. AC3/4: TestNotifyOutcomes and TestNotifyInterruption verify private single-object JSON, missing executable=3, partial/failure=4, timeout/cancellation=5, no-match=3 and unsupported routes; provider stdin and raw output are isolated. AC6: README, configuration/design docs, help, templates, examples, config output updated and CI green. Single independent review found idle-stdin cancellation hang and post-delivery JSON-write misclassification; both fixed with regressions. Direct binary checks confirm SIGTERM with open idle stdin exits config/cancelled promptly, and read-only stdout after delivery retains accepted exit 0 with safe warning. Implementation commit 6ceeaf650ae3080232fc1bda4331bec170665934 fast-forward merged into main. task test passed on main. Owned tell-2-notify worktree and branch removed through Worktrunk; exact Herdr workspace absence verified. No push. No blockers or remaining steps.

Review: no defects found; no code changes. Built-binary probes with local fixtures: Apprise argv keeps dash-leading body and tag as option values (Click options consume the next token), always passes --body so stdin is never read (fixture saw empty stdin); exits 3/4/5/2 map to not_sent/failed/uncertain/failed; provider timeout maps to uncertain/timeout; provider stderr never surfaces; stdin is not read without '-'; empty stdin is config/empty_message. Apprise exit enum rechecked against upstream cli.py (ctx.exit(status)). Observation, not fixed (TELL-10 scope): notify - reads stdin unbounded and gitleaks scans the whole input before truncation, ~3s per MB of high-entropy text (5 MB took 17s, 200 MB exceeded 2 min), so piping large logs is slow and memory-heavy. task ci passed.

## Summary

Implemented stderr/Apprise notify with explicit input selection, private structured outcomes and bounded provider execution. Verified with task ci, local provider fixtures, direct signal/output-failure checks and independent review (two findings fixed). Committed 6ceeaf6, merged main, removed owned worktree/branch/workspace; no push.
