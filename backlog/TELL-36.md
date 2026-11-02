---
id: TELL-36
title: Example Noon-timed escalation tiers with local acknowledgement
status: done
type: docs
labels: [notify, escalation]
created: 2026-11-02T03:18:12Z
updated: 2026-11-02T03:36:56Z
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

- [x] raise.sh delivers tier 1 at once and schedules later tiers as Noon temporary jobs whose names are derived from a hash of the incident key
  Evidence: 616883f examples/escalate/raise.sh: tier 1 via tell notify at once; tiers 2-3 via noon --user add inc-<sha256[:12]>-tN --temporary --key inc-<hash> --in DELAY. Isolated Noon 0.15.0 run: tier 1 logged 20:27:59 at raise; jobs inc-5b96402eca72-t2/t3 created.
- [x] Against a real Noon scheduler with 1m waits: without acknowledgement tier 2 is delivered; ack.sh before the wait prevents it; a second raise.sh for the same key creates no new jobs
  Evidence: Real noon serve (NOON_STATE_DIR temp), TIER2=1m TIER3=2m: unacked 'db-primary down' tier 2 ran 20:28:59 (run 1 succeeded), tier 3 20:29:59; ack.sh 'disk full' before due cancelled runs 3,4, nothing delivered; second raise.sh printed 'already raised; nothing sent' with created:false, no new jobs.
- [x] Noon downtime across a tier due time is tested and the observed behavior (late run or skip) is documented
  Evidence: Stopped noon serve 20:30:13 with 'net down' tier 2 due 20:31:10; nothing sent while down; restarted 20:31:44; run 7 started 20:31:45 succeeded: runs late, once. Documented in examples/escalate/README.md Limits table and docs/design.md Escalation.
- [x] Docs cover the TERM/uncertain race on acknowledgement, local-only acknowledgement, and that Tell stores no state; task ci passes
  Evidence: README + design.md cover TERM/uncertain ack race, recall impossibility, local-only ack, Tell stores no state. task ci passed on rebased 616883f (tests, race, vet, staticcheck, smoke, integration); shellcheck clean.

## Summary

Added examples/escalate/raise.sh and ack.sh (Noon temporary-job tiers keyed by hashed incident key), merged into TELL-35's escalate README as 'Timed tiers with Noon', extended design.md Escalation. Verified against a real isolated Noon 0.15.0 scheduler; commit 616883f fast-forwarded onto main.
