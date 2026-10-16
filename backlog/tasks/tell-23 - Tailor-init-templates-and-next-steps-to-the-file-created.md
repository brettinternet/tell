---
id: TELL-23
title: Tailor init templates and next steps to the file created
status: To Do
assignee: []
created_date: '2026-10-16 23:24'
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
- [ ] #1 Templates list every implemented notify route and send transport and contain no guidance irrelevant to that file.
- [ ] #2 After create or keep, the next step matches the file: shared file says to commit it and points to `tell init --user` for routing; user file says to set notify.via then run `tell doctor`.
- [ ] #3 README and docs/configuration.md init sections match; task ci passes.
<!-- AC:END -->
