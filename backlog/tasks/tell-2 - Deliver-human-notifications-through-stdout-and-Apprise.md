---
id: TELL-2
title: Deliver human notifications through stdout and Apprise
status: To Do
assignee: []
created_date: '2026-10-16 20:28'
updated_date: '2026-10-16 20:29'
labels: []
dependencies:
  - TELL-1
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
People and Noon jobs need a stable notify command rather than duplicating provider arguments in scripts. Scope: internal/cli, internal/config only for exercised new options, a concrete internal/delivery package, command tests and fixture executables, docs, examples, and smoke coverage. Support stdout, Apprise, and the desktop route using Apprise macosx. Preserve the config contract; let Apprise own destinations and credentials. No scheduler, queue, automatic retry, provider SDK, arbitrary shell template, or plugin registry. See docs/design.md and docs/configuration.md.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 task test and task smoke exercise tell notify with a literal message and stdin input; stdout writes the message without launching a provider.
- [ ] #2 CLI tests use a real local fixture executable to verify exact argv for title/body/config/tag/format and native APPRISE_* inheritance when Tell leaves an option unset; no actual service is contacted.
- [ ] #3 Execution tests prove timeout and cancellation are bounded, failure/start-error/uncertain-delivery outcomes are not reported as successful delivery, and diagnostics do not echo secret URLs or message bodies.
- [ ] #4 task ci passes and help/docs explain the distinction between provider acceptance and human acknowledgment, including the optional external Apprise dependency.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->
