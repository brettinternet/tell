---
id: TELL-36
title: Example Noon-timed escalation tiers with local acknowledgement
status: todo
type: docs
labels: [notify, escalation]
created: 2026-11-02T03:18:12Z
updated: 2026-11-02T03:18:12Z
---

## Context

TELL-33 asked whether Noon could own escalation timers and stop them by key. Noon cannot cancel by key, but it can cancel by job name, and temporary jobs cover everything else with no Noon or Tell changes:

- `noon add inc-<h>-t2 --temporary --in 10m --for 30m --key <incident> --sh 'tell notify ...'` schedules a durable delayed tier. Noon keeps its state in SQLite, so the tier survives a restart.
- Reusing the same `--key` returns the existing job (`created: false`), so raising an incident twice does not duplicate tiers.
- `"every 5m" --temporary --for 2h` repeats the last tier.
- `noon cancel inc-<h>-t2 inc-<h>-t3` acknowledges or resolves the incident; temporary jobs also end on their own at `--for`.
- Job names must match `^[a-z][a-z0-9_-]{0,63}$`, so the incident key is hashed into the name.

Each tier runs `tell notify`, and that notify may itself be a TELL-30 cascade, for example desktop, then phone, then Pushover emergency.

## Scope

An example `examples/escalate/` with `raise.sh INCIDENT_KEY TITLE` (creates the tier jobs from a policy written as variables at the top of the script) and `ack.sh INCIDENT_KEY` (cancels every tier job for that incident). Acknowledgement is local only. Document the races: acknowledging while a tier is running stops it with TERM, so that alert is uncertain; acknowledging after a tier has been delivered cannot recall it.

## Non-goals

Tell or Noon code changes; a policy schema in Tell config; acknowledgement through tell listen (TELL-33); multi-person on-call; quiet hours; state kept by Tell.

## Dependencies

None in Tell. Requires the Noon temporary-job features documented in ../noon/docs/cli.md.

## Modified-file boundaries

examples/escalate/**, examples/README.md, docs/design.md (escalation section).

## Verification

Run the example against a real Noon scheduler with short waits (`--in 1m`) and a stderr or command notify route. Check that tier 1 is delivered at once; tier 2 is delivered after the wait if nothing acknowledges it; running `ack.sh` before the wait prevents tier 2; running `raise.sh` twice for the same key creates no new jobs (`created: false`). Stop Noon while tier 2 is queued, restart it after the due time, and record whether the tier runs late or is skipped (see Noon decision-5); the docs must state that behavior. Run `task ci`.

## Acceptance criteria

- [ ] raise.sh delivers tier 1 at once and schedules later tiers as Noon temporary jobs whose names are derived from a hash of the incident key
- [ ] Against a real Noon scheduler with 1m waits: without acknowledgement tier 2 is delivered; ack.sh before the wait prevents it; a second raise.sh for the same key creates no new jobs
- [ ] Noon downtime across a tier due time is tested and the observed behavior (late run or skip) is documented
- [ ] Docs cover the TERM/uncertain race on acknowledgement, local-only acknowledgement, and that Tell stores no state; task ci passes
