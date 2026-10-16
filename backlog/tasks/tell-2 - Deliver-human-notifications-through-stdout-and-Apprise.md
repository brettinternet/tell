---
id: TELL-2
title: Deliver human notifications through stdout and Apprise
status: To Do
assignee: []
created_date: '2026-10-16 20:28'
updated_date: '2026-10-16 20:47'
labels: []
dependencies:
  - TELL-1
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
Why: people and Noon jobs need one stable notify command instead of duplicating provider argv in scripts. This task defines the notify contract (message input, title, JSON result, outcome semantics) and the bounded subprocess helper that TELL-3, TELL-4, and TELL-5 reuse.

Context not recoverable from the code:
- Noon hooks receive a JSON run summary on stdin and append hook stdout/stderr to the run log (../bump/docs/hooks.md). Tell must never consume stdin unless the caller explicitly selects it.
- Apprise CLI: --title/-t, --body/-b (reads stdin when omitted), --config/-c, --tag/-g, --input-format/-i text|markdown|html. Without --tag only untagged services are notified. With no --config and no URLs, Apprise uses APPRISE_CONFIG_PATH/default config paths or APPRISE_URLS. Positional URLs override --config and --tag. Non-zero statuses include partial delivery, no matching service/tag, and timeout; only exit 0 is success.
- The desktop route invokes Apprise with only the macosx:// URL (requires terminal-notifier on macOS); apprise.config and apprise.tag do not apply. On other platforms desktop fails visibly. Linux desktop delivery is out of scope.
- Bootstrap text says nothing is sent: README implemented/not-implemented lines, the CLI root description, the init template header, the trailer of tell config output, docs/design.md, docs/configuration.md, and the smoke assertion that notify fails.

Scope: notify command in internal/cli; new internal/delivery package (stdout, Apprise, desktop, shared subprocess helper); internal/config only for options this task exercises; CLI tests with local fixture executables; integration/smoke_test.go; README, docs, examples, and the init template.

Non-goals: Herdr or hcom routes, scheduling or queues, automatic retries, fallback to another route, provider SDKs, shell templates, plugin registry, Apprise URLs or credentials in Tell config.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test and task smoke exercise tell notify with a literal message and with stdin selected explicitly (for example -); without explicit selection notify never reads stdin, an empty message fails before any provider runs, and the stdout route prints the message without launching a provider.
- [ ] #2 CLI tests use a real local fixture executable to verify exact Apprise argv for title, body (always passed with --body), config, tag, and format; unset Tell options are omitted so native APPRISE_* behavior applies; desktop passes only macosx:// and fails visibly on non-macOS hosts. No real service is contacted.
- [ ] #3 Provider subprocesses never inherit Tell's stdin; timeout and cancellation are bounded; provider non-zero exits (including partial and no-match), start errors, and timeout-uncertain outcomes exit non-zero and are never reported as delivered; diagnostics never echo message bodies or credential URLs.
- [ ] #4 tell --json notify emits one schema_version 1 result object for success and failure, without the message body.
- [ ] #5 README, docs, CLI help, the init template, and tell config output describe notify as implemented, state that provider acceptance is not human acknowledgment, and list Apprise as an optional external dependency; task ci passes.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
