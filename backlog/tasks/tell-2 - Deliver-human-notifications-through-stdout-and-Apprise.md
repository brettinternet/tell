---
id: TELL-2
title: Deliver human notifications through stderr and Apprise
status: To Do
assignee: []
created_date: '2026-10-16 20:28'
updated_date: '2026-10-16 21:43'
labels: []
dependencies:
  - TELL-1
  - TELL-7
references:
  - 'https://github.com/caronc/apprise/blob/master/apprise/cli.py'
documentation:
  - docs/design.md
  - docs/configuration.md
  - ../bump/docs/hooks.md
type: feature
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: people and Noon jobs need one stable notify command instead of duplicating provider argv in scripts. This task defines the notify contract (message input, title, notification type, JSON result, outcomes) and the bounded subprocess helper that the desktop, command, Herdr, hcom, and Noon hook tasks reuse.

Context not recoverable from the code:
- The outcome contract (exit codes 0-5: accepted, config, usage, not_sent, failed, uncertain) and JSON result shape are defined in docs/design.md. Every route maps into it; unknown provider exits map to failed.
- The built-in default route writes to stderr, not stdout: a calling script's stdout may be captured as data, and --json needs stdout for exactly one object. The bootstrap config value notify.via: stdout becomes stderr.
- Noon hooks receive a JSON run summary on stdin and append hook stdout/stderr to the run log (../bump/docs/hooks.md). Tell must never consume stdin unless the caller explicitly selects it.
- Apprise CLI: --title/-t, --body/-b (reads stdin when omitted), --config/-c, --tag/-g, --input-format/-i text|markdown|html, --notification-type/-n info|success|warning|failure. Without --tag only untagged services are notified. With no --config and no URLs, Apprise uses APPRISE_CONFIG_PATH/default config paths or APPRISE_URLS. Positional URLs override --config and --tag. Statuses include partial delivery, no matching service/tag, and timeout; confirm their numeric exit codes before mapping them.
- Bootstrap text says nothing is sent: README implemented/not-implemented lines, the CLI root description, the init template header, the trailer of tell config output, docs/design.md, docs/configuration.md, and the smoke assertion that notify fails.

Scope: notify command in internal/cli; new internal/delivery package (stderr, Apprise, shared subprocess helper); internal/config for the stderr rename and options this task exercises; CLI tests with local fixture executables; integration/smoke_test.go; README, docs, examples, and the init template.

Non-goals: desktop, command, Herdr, or hcom routes; message normalization and secret redaction (separate tasks); scheduling or queues; automatic retries; fallback to another route; provider SDKs; shell templates; plugin registry; Apprise URLs or credentials in Tell config.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test and task smoke exercise tell notify with a literal message and with stdin selected explicitly (for example -); without explicit selection notify never reads stdin, an empty message fails before any provider runs, and the default stderr route writes the message to stderr without launching a provider, leaving stdout untouched.
- [ ] #2 CLI tests use a real local fixture executable to verify exact Apprise argv for title, body (always passed with --body), config, tag, format, and notification type (info, success, warning, failure; default info); unset Tell options are omitted so native APPRISE_* behavior applies. No real service is contacted.
- [ ] #3 Results follow the outcome table in docs/design.md: tests cover a missing executable (3 not_sent), provider failure and partial delivery (4 failed), and timeout and cancellation after start (5 uncertain). Provider subprocesses never inherit Tell's stdin, and diagnostics never echo message bodies, credential URLs, or raw provider output.
- [ ] #4 tell --json notify emits exactly one schema_version 1 object with outcome and route, plus error code, message, and reason on failure, for every route; it never contains the message body.
- [ ] #5 Without --title, the title defaults to the selected project's directory name; notify.via accepts stderr instead of stdout in config, docs, examples, and the init template.
- [ ] #6 README, docs, CLI help, the init template, and tell config output describe notify as implemented, state that provider acceptance is not human acknowledgment, and list Apprise as an optional external dependency; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
