---
id: DRAFT-2
title: Explore explicit trust for shared repository routing
status: Draft
assignee: []
created_date: '2026-10-17 17:31'
labels: []
dependencies: []
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context
Auto-discovered tell.yaml accepts shared settings but rejects personal routing, including notify.via: desktop. Users must move routing to personal configuration or explicitly select the file with --config on each invocation. There is no persistent way to approve committed routing defaults.

Noon offers enable to approve current shared job definitions, with changed definitions requiring reapproval. Consider a similar explicit approval workflow for Tell. Proposed names are tell trust / tell untrust because Tell delivers one invocation and exits rather than enabling background work. Names and semantics are not settled.

## Candidate scope
Allow users to explicitly approve shared repository routing, revoke approval, and inspect approval status. Preserve personal configuration precedence and safe no-argument inspection. Approval must not let subsequent repository changes silently redirect delivery or introduce executable command routing.

## Questions for refinement
- Should this be trust/untrust, enable/disable, or another interface?
- Which currently personal-only fields may be approved: route selection only, or also command argv, provider paths, identities, secret policy, and MCP target authority?
- What changes invalidate approval: the whole file or only security-sensitive settings?
- Where should approval live, and is its scope a checkout, repository, or shared worktree group?
- How should unapproved or changed routing behave: reject configuration or ignore routing with an actionable diagnostic?
- How does approval interact with explicit --config, environment overrides, symlinks, and MCP callers?
- Is persistent approval state an acceptable exception to the current no-durable-state design constraint?

## Candidate acceptance for refinement
- An unapproved clone cannot choose personal routing automatically.
- Explicit approval permits the agreed shared fields without repeated --config.
- Relevant changes require reapproval before delivery; revocation removes that authority.
- Personal overrides retain precedence, and inspection sends nothing or starts no services.
- Diagnostics explain the approval state and next action without exposing secrets or raw command configuration.

## Boundaries and references
Likely surfaces: internal/config, internal/cli, configuration schemas, README.md, docs/configuration.md, docs/design.md, and focused security regression tests. No scheduling, queues, background services, or changes to Noon. Related completed work: TELL-7, TELL-15, TELL-24; reference ../noon approval behavior. No known prerequisite task; product and trust-model decisions remain unresolved.

Draft idea only. Refine before promotion or implementation.
<!-- SECTION:DESCRIPTION:END -->
