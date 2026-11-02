---
id: TELL-41
title: Request Herdr upstream report of detached sessions for notifications
status: done
type: chore
labels: [notify, escalation]
created: 2026-11-02T03:42:41Z
updated: 2026-11-02T04:28:29Z
---

## Context

TELL-40 maps Herdr's `notification show` reasons to Tell outcomes. Herdr defines `no_foreground_client`, but in Herdr 0.9.5 a session with no attached client (only a `herdr server` running, with no `herdr session attach` process) still returns `{"reason":"shown","shown":true}`. Herdr's API has no other way to tell whether anyone is watching: `herdr status client` describes the local binary, and `herdr api snapshot` reports focus even for detached sessions. Clients may attach over SSH, so local process checks aren't reliable.

## Scope

Draft and, after the user approves, file one upstream request asking Herdr for either:

1. `notification show` returning `no_foreground_client` (`shown: false`) when no client is attached to the session, or
2. attached-client information in `herdr api snapshot` (count, plus last input time if available).

Option 1 is preferred because TELL-40 would then handle it with no Tell changes.

## Non-goals

Tell code changes, workarounds based on processes or sockets, filing anything without the user's approval.

## Dependencies

None. TELL-40 benefits once Herdr ships the change.

## Modified-file boundaries

None in the repository. Backlog only (refs and notes on this task).

## Verification

Reproduce on the current Herdr version first: create a detached session and run `herdr --session NAME notification show "probe" --body "probe" --sound none`, then record the output. Find Herdr's issue tracker from its official docs; don't guess the URL.

## Acceptance criteria

- [x] A draft request is written and approved by the user before anything is filed upstream
  Evidence: No request drafted: reproduction on herdr 0.9.5 showed the premise is already handled upstream; user reviewed findings and chose to close without filing (2026-11-02).
- [x] The request includes the Herdr version, the reproduction (notification show to a session with no attached client returns reason shown), and the two alternatives (return no_foreground_client for detached sessions, or expose attached-client count and last input time in api snapshot)
  Evidence: Herdr 0.9.5 repro: never-attached headless session -> reason no_foreground_client, shown false; attached client -> reason shown; after client killed -> no_foreground_client, shown false. Upstream src/server/headless/notifications.rs:244 returns NoForegroundClient when no shell client is connected, so alternative 1 already exists; no request needed.
- [x] After the user approves, the request is filed in Herdr upstream issue tracker and its URL is added to this task as a ref; or the task records that the user declined
  Evidence: User declined filing (premise not reproducible; option 1 already shipped in 0.9.5). Nothing filed upstream; no ref added.

## Notes

Repro on herdr 0.9.5 (2026-11-02) does NOT confirm premise. Fresh headless session (herdr --session tell41-probe server, never attached): notification show -> {reason:no_foreground_client, shown:false}. Client attached in a Herdr pane -> {reason:shown}. Attach client killed -> {reason:no_foreground_client, shown:false}. Source: herdrdev/herdr src/server/headless/notifications.rs:244 returns NoForegroundClient when no shell client is connected, so option 1 already exists. Original 'shown' observation was likely a session with a live attach process (e.g. scratch had 'herdr session attach scratch' running) that nobody was looking at. Remaining gap is attached-but-unattended (option 2: last input time). Herdr routes feature requests to GitHub Discussions (https://github.com/herdrdev/herdr/discussions per .github/ISSUE_TEMPLATE/config.yml); issues are bug-only. Awaiting user decision: decline/close, or file a Discussion for attached-client idle info.

Resolution: user chose to close without filing (2026-11-02). No repo code changes.
