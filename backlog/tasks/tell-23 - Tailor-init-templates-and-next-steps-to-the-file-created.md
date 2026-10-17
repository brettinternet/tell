---
id: TELL-23
title: Tailor init templates and next steps to the file created
status: Done
assignee:
  - '@brett'
created_date: '2026-10-16 23:24'
updated_date: '2026-10-17 01:41'
labels: []
dependencies:
  - TELL-15
modified_files:
  - internal/config/init.go
  - internal/cli/cli.go
  - README.md
  - docs/configuration.md
priority: low
type: enhancement
ordinal: 16000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: init output and templates give wrong or irrelevant guidance. Every init variant prints "Ignore .tell.yaml in Git. Next: tell config check", including `tell init` (shared, committed tell.yaml) and `tell init --user`. Both templates say notify uses "stderr or Apprise", omitting desktop, herdr, and command; the user template tells people to ignore .tell.yaml in Git. A new user who runs `tell init` expecting to configure notifications gets a shared file where routing is rejected, with no pointer to `tell init --user`.

Depends on TELL-15, which may remove the .tell.yaml layer and `init --local`.

Non-goals: interactive prompts, writing route values from flags, overwriting existing files, --json output changes.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 Templates list every implemented notify route and send transport and contain no guidance irrelevant to that file.
- [x] #2 After create or keep, the next step matches the file: shared file says to commit it and points to `tell init --user` for routing; user file says to set notify.via then run `tell doctor`.
- [x] #3 README and docs/configuration.md init sections match; task ci passes.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Tailor shared/user templates and create/keep guidance without changing JSON or overwrite safety. 2. Align init documentation and exercise both variants, then run task ci. 3. Commit, merge to main, record completion, and clean up the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented and fast-forward merged bb87005 into main. Verification: go test ./internal/config ./internal/cli and task ci passed (unit, race, smoke, vet, Staticcheck, security). LSP clean for both changed Go files. Isolated built-binary checks passed for shared/user create and keep guidance, all route/transport names, unchanged JSON schema, byte-preservation, and config check. Initial ad hoc assertions corrected for macOS canonical paths and existing schema_version field; no product defect. One scoped diff review found no defects. README and configuration docs agree. task check:staged passed before implementation commit. Worktrunk removed owned worktree and branch; exact Herdr workspace disappearance verified. No blockers or remaining implementation steps.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Tailored shared and personal init templates and next steps for create/keep, preserving JSON and overwrite safety. Verified focused tests, isolated CLI execution and task ci; merged bb87005 to main and cleaned up the worktree.
<!-- SECTION:FINAL_SUMMARY:END -->
