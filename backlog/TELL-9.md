---
id: TELL-9
title: Normalize outbound message text before delivery
status: done
type: enhancement
assignees: ['@pi']
labels: [reviewed]
blocked_by: [TELL-2]
rank: a1e1e1e1e1e1e1e1c
refs: [docs/design.md]
created: 2026-10-16T21:41:00Z
updated: 2026-10-16T23:20:00Z
---

Why: titles and bodies often come from job output or logs. Terminal-rendered routes (stderr, Herdr) interpret escape sequences, so untrusted text can rewrite the screen or spoof content; bidi controls can disguise text in banners; huge bodies are truncated or rejected inconsistently by providers. One preparation step shared by every route keeps behavior predictable.

Context: config values already reject NUL, ESC, CR, and LF (internal/config). Unharness bounds message size and treats content as inert data (../unharness/docs/threat-model.md).

Scope: one shared preparation step in internal/delivery or a small sibling package, CLI wiring, tests, docs.

Non-goals: markdown or HTML sanitization (format is chosen by personal config), secret scanning (separate task), translation or reformatting of content.

## Acceptance criteria

- [x] Before any route runs, NUL in a title or body fails with exit 1; other C0 and C1 controls, terminal escape sequences, and Unicode bidi override and isolate controls are removed while newline and tab are preserved. Tests cover an ANSI escape sequence and a bidi override.
- [x] Titles and bodies are capped at documented byte limits on UTF-8 boundaries with a visible truncation marker; tests cover a multibyte boundary.
- [x] Docs state the limits and the removal rules; task ci passes.

## Definition of done

- [x] Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.

## Plan

1. Prepare titles and bodies centrally in delivery.Notify: reject NUL before filtering, strip terminal sequences/control and bidi characters, cap UTF-8 titles at 256 bytes and bodies at 16384 bytes including a visible marker; reject bodies empty after preparation. 2. Exercise CLI stderr and real fixture Apprise boundaries, NUL no-dispatch, and multibyte truncation. Document exact rules. 3. Run focused tests and task ci, one independent security review, commit implementation, merge main, finalize backlog evidence, and remove the owned worktree.

## Notes

Implemented central preparation before dispatch. Evidence: go test ./internal/cli ./internal/delivery passed; task ci passed twice (unit, race, built-binary smoke, vet, Staticcheck, Gitleaks, govulncheck). CLI tests exercise title/body ANSI and C1 sequences, bidi overrides/isolates, newline/tab preservation, malformed UTF-8 replacement, NUL including beyond limits with no provider launch, empty-after-filtering rejection, and multibyte title/body truncation within 256/16384 bytes. Direct built-binary stdin checks confirm ANSI/bidi removal and config/invalid_message exit 1 for NUL. Independent review identified ESC followed by ignored DEL/C0 leaking sequence payload; fixed and added regression inputs. Direct reruns now produce safetail and safered, and full task ci passes. LSP diagnostics clean. Docs state exact removal rules and limits. Review completion and commit/integration pending.

Delivery complete: implementation fcdda0e merged to main as 03e08e8, preserving concurrent scheduler-neutral documentation changes. task check:staged passed before both commits; task ci passed again on merged main. Single independent reviewer concluded with only the DEL/C0 escape finding, already corrected and executably revalidated by parent. Owned worktree tell-9-normalize and its branch removed via Worktrunk; associated Herdr workspace wDW verified gone. No push performed, no remaining blocker or resumable implementation step.

Review: no findings. 30s fuzz of normalize (6.5M execs, limits 12-4096) never produced output over the limit, invalid UTF-8, C0 except newline/tab, C1, or bidi override/isolate characters, and normalization is idempotent, so scanning before truncation remains sound. NUL rejection runs before any route.

## Summary

Normalize all notification titles/bodies before dispatch: reject NUL, remove terminal/control/bidi sequences, preserve ordinary newline/tab text, and cap valid UTF-8 output with a visible marker. Verified real CLI/provider boundaries, multibyte truncation, review regression, and full task ci on merged main. Implementation fcdda0e; merge 03e08e8. Worktree/branch/workspace cleaned.
