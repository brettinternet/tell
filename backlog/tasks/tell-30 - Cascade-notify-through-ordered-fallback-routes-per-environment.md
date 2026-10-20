---
id: TELL-30
title: Cascade notify through ordered fallback routes per environment
status: To Do
assignee: []
created_date: '2026-10-20 19:35'
labels:
  - notify
dependencies: []
documentation:
  - docs/design.md
  - docs/configuration.md
  - examples/fallback/README.md
  - docs/listening.md
priority: medium
type: feature
ordinal: 23000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
## Context
The same personal Tell config is synced across several environments (laptops, servers, headless hosts) whose capabilities differ. One machine has an Apprise config with a phone-push service, another has only chat, another has no Apprise at all, a headless box has no desktop. Currently notify.via picks exactly one route, so a shared config either fails on machines that lack it or must be forked per machine. The user wants notify to silently cascade to the next available method, including through several Apprise targets, with optional visible escalation when a fallback was used.

Apprise does not do this: it fans out to every service matching the config/tag concurrently and reports partial failure. Apprise tag expressions like "phone, chat" are OR fan-out, not ordered fallback. Tell must own ordering, but must never store Apprise URLs or credentials; each Apprise step selects services by tag (and optionally a different Apprise config file).

This reverses the current "no fallback routes" stance for notify only. The safety rule from examples/fallback/deliver.sh stays the core contract: advance only when the outcome promises nothing was delivered. docs/design.md already states a dropped alert costs more than a duplicate, which justifies an explicit opt-in to also advance on failed. uncertain never advances: a slow provider plus fallback would duplicate every alert. send stays single-route because a duplicated request can mean two agents doing the same work.

## Settled interface
- notify.via accepts its current string form (unchanged meaning) or an ordered list of steps. A step is a route name (stderr, desktop, apprise, herdr, command) or a single-key map with options for that route: apprise {config, tag, format}, herdr {session}. stderr, desktop, and command take no step options (command uses notify.command). Repeated routes are allowed, e.g. two apprise steps with different tags. Unknown keys, multi-key steps, and empty lists are config errors.
- Precedence: step option > top-level apprise/herdr block for that step. One-shot overrides (flags, TELL_* environment, MCP agent targets) of apprise tag/config/format or herdr session keep top precedence and apply to every step of that route. Overriding via (flag, TELL_NOTIFY_VIA, MCP) selects that single route and disables the cascade. A list in a higher config layer replaces a lower one wholesale (no merging). The list is personal routing, rejected in shared tell.yaml like notify.via is now.
- notify.fallback_on: list, default [not_sent]; the only other allowed value is failed; uncertain, config, and usage are rejected by validation. notify.fallback_notice: none (default, silent) | title. Both are valid only with a list via.
- Config and usage errors are detected for every step before any step runs, so a cascade never stops midway on a bad config.
- A step advances only when its outcome is in fallback_on. The command exits with the outcome and exit code of the last attempted step; accepted stops the cascade. All steps not_sent gives exit 3.
- --timeout is one budget for the whole cascade; each step gets the remaining time. If the budget is exhausted before a step starts, stop with not_sent/timeout_budget (when all prior outcomes were not_sent).
- fallback_notice: title prefixes the delivered title on any step after the first with the first step label and reason, e.g. "Fallback from apprise:phone (no_match): Build finished". The prefixed title goes through the normal preparation pipeline. Body text is unchanged.
- An apprise step whose effective config file does not exist returns not_sent/missing_config without running Apprise (currently Apprise exits 1, mapped to failed, which would wrongly stop the cascade). This also applies to single-route apprise.
- JSON output keeps the current top-level fields, taken from the final step, and adds an attempts array of {route, label, outcome, reason}, never message text. Human output stays silent on skipped steps when the result is accepted; on a non-accepted result it lists the reason for each attempt.
- doctor shows the cascade in order. A step that is not ready is "unavailable here", not an error; the notify side is ready (exit 0) if any step is ready. JSON adds the ordered steps. Same probes as now, no sending.
- Listener notify actions without a static via inherit the personal cascade with fallback_on/notice; a static via stays single-route. The per-action timeout bounds the whole cascade. Dry-run still probes nothing. The MCP notify tool behaves like the CLI and returns attempts.
- Config inspection (tell, tell config) shows the cascade without secrets or raw provider config.

## Non-goals
send fallback; retries of the same step; advancing on uncertain; per-step timeouts; Apprise URLs or credentials in Tell config; per-step command argv; time-based or acknowledgement-based escalation (see the separate escalation draft); new transports; persistent state.

## Dependencies
None. Builds on completed TELL-2 (Apprise), TELL-11 (command), TELL-12 (doctor), TELL-14 (MCP), TELL-28 (listener notify actions).

## Modified-file boundaries
internal/config/**, internal/delivery/**, internal/cli/** (notify, doctor, mcp, output), internal/listen/** (inheritance only), tell.schema.json, tell.shared.schema.json, integration/smoke_test.go, README.md, docs/design.md, docs/configuration.md, docs/listening.md, examples/fallback/**, examples/README.md.

## Verification
Use existing local provider fixtures (fake apprise/herdr/command executables), not mocks. Tests cover only the critical paths: advance/stop rules (especially no advance on failed by default and never on uncertain, preventing duplicates), strict config parsing/validation and precedence, missing_config, and the timeout budget. Run focused Go tests, then task ci.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A list notify.via (bare-name and single-key map steps; repeated apprise steps with different tags/configs) parses with strict validation: unknown keys, multi-key steps, empty lists, uncertain/config/usage in fallback_on, fallback options with a string via, and a list in shared tell.yaml are config errors reported before any delivery. A string via behaves exactly as before.
- [ ] #2 Against local fixtures, a step with not_sent (Apprise exit 3 no_match, missing executable, missing Herdr session, unsupported platform, missing Apprise config file) advances to the next step; accepted stops; failed stops unless fallback_on includes failed; uncertain always stops. Exit code and top-level JSON reflect the last attempted step; all not_sent gives exit 3.
- [ ] #3 Step options override top-level apprise/herdr settings; tag/config/format/session flags, TELL_* env, and MCP agent overrides apply to every step of that route; a via override runs only that route with no cascade. Precedence is covered by tests.
- [ ] #4 A missing effective apprise.config file yields not_sent/missing_config without running Apprise, for both cascade steps and single-route apprise, and doctor reports it.
- [ ] #5 --timeout bounds the whole cascade; exhaustion before a step starts yields not_sent/timeout_budget and runs no further steps.
- [ ] #6 fallback_notice none sends unchanged text; title prefixes later-step titles with the first step label and reason through normal preparation. JSON includes an attempts array (route, label, outcome, reason) and never message text; human output is silent about skipped steps on success and lists attempt reasons on failure.
- [ ] #7 doctor lists cascade steps in order, labels unready steps as unavailable here, and exits 0 when any step is ready, without sending. Listener notify actions without static via inherit the cascade within the per-action timeout; dry-run probes nothing; MCP notify returns attempts. send behavior is unchanged.
- [ ] #8 Schemas, README, docs/design.md (replacing the no-fallback-routes statement for notify), docs/configuration.md, docs/listening.md, and examples/fallback document the cascade, advance rules, Apprise-tag cascading, and why send has no fallback. Focused tests and task ci pass with recorded evidence.
<!-- AC:END -->
