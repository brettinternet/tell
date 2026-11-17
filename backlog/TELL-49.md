---
id: TELL-49
title: Suggest corrections for mistyped config keys, values, and flags
status: todo
type: feature
priority: medium
labels: [cli, config]
created: 2026-11-17T19:51:46Z
updated: 2026-11-17T19:51:46Z
---

## Context

Strict config and flags are right, but typos stop with no hint:

```text
tell: config typo.yaml: line 3: unknown field "notify.vai"
tell: config typo2.yaml:3: notify.via must be stderr, desktop, alert, ...    # for "dekstop"
tell: flag provided but not defined: -vai; run tell notify --help
```

Mistyped root commands already get suggestions (TELL-16, `urfave.SuggestCommand`).

## Scope

- Append `did you mean "X"?` when exactly one close candidate exists (small edit distance) for unknown YAML keys (against the valid keys at that path), enum values (for example `notify.via`, `send.via`, notify type, send intent), and unknown flags of the invoked command (urfave `SuggestFlag`).
- Suggest only names from the schema; never echo the typed value beyond what the error already shows.
- Exit codes and error codes are unchanged.

## Non-goals

Auto-correction, suggestions for free-form values (paths, recipients, tags, URLs), and changes to command suggestions.

## Dependencies

None. Related: TELL-16.

## Modified-file boundaries

internal/config/yaml.go, internal/config/config.go (enum validation), internal/cli/cli.go (flag errors), existing tests in internal/config and internal/cli.

## Verification

Built-binary checks of the three errors above; `task ci`.

## Acceptance criteria

- [ ] notify.vai suggests notify.via, notify.via dekstop suggests desktop, and tell notify --vai suggests --via, with exit codes unchanged (1 config, 2 usage); covered by tests
- [ ] Distant typos and free-form values get no suggestion, and suggestions never echo the typed value; covered by tests
- [ ] task ci passes
