---
id: TELL-45
title: Link notifications to a URL with notify --url
status: todo
type: feature
priority: high
labels: [notify]
created: 2026-11-17T19:51:37Z
updated: 2026-11-17T19:51:37Z
---

## Context

Scripts send pointers, not content (examples/README.md patterns), so links get pasted into message bodies: examples/wait-for/wait-pr-checks.sh, examples/ci-to-agent/watch-pr.sh (twice), and both examples/github-actions workflows. A desktop click returns to the origin terminal (TELL-43), not to the failed run, and Pushover shows the link as text instead of its native link.

## Scope

- `tell notify --url URL` (CLI only). Accept absolute `http` or `https` URLs with a host, no userinfo, no whitespace or control characters, at most 512 bytes. Anything else is a usage error (exit 2) before any route runs. No config key or environment variable: the URL describes one message.
- Message preparation scans the URL with the title and body. A URL containing a detected secret is dropped and counted in `redactions`; under `secrets.policy: reject` the message is rejected as for body secrets. The URL is never truncated.
- Route behavior:

  | Route | URL |
  | --- | --- |
  | desktop with Tell.app | a click opens the URL (`/usr/bin/open URL` click action) instead of the origin action; a personal `notify.desktop.on_click` still wins and receives `TELL_URL` |
  | pushover | native `url` field |
  | command | appended as the last body line and set as `TELL_URL` |
  | stderr, desktop via osascript or notify-send, alert, apprise, shoutrrr, imessage, herdr | appended as the last body line |

- Each cascade step applies its own route's rule. JSON results do not echo the URL.
- Replace pasted links with `--url` in the examples listed above.

## Non-goals

`url` in listener notify actions (event authors are untrusted; a template could plant a phishing link), MCP notify (agent-chosen links), `send`, action buttons, Pushover `url_title`, native Shoutrrr per-service parameters such as ntfy `click`, Linux click actions, and non-http schemes.

## Dependencies

None. Related: TELL-43 (click actions), TELL-42 (Pushover route), TELL-10 (redaction).

## Modified-file boundaries

internal/cli (cli.go flag, notify.go), internal/delivery (delivery.go message, prepare.go, desktop.go, pushover.go, command delivery, cascade), existing tests in those packages, README.md, docs/design.md, docs/configuration.md, and the examples listed above.

## Verification

Real Mac click test with Tell.app installed; Pushover through a local fixture server; built-binary checks of each appended route; `task ci`.

## Acceptance criteria

- [ ] With Tell.app installed, clicking a notification sent with --url https://... opens that URL in the default browser after tell notify has exited; verified on a real Mac and recorded
- [ ] Pushover requests carry the URL as the url field (local fixture server); stderr, osascript or notify-send desktop, alert, apprise, shoutrrr, imessage, and herdr get it as the last body line; command gets it in the body and TELL_URL; covered by tests
- [ ] Relative, file:, javascript:, userinfo, over-512-byte, and control-character URLs exit 2 and run no route; covered by a test
- [ ] A URL containing a detected secret is never delivered: dropped and counted in redactions under redact, rejected under secrets.policy reject; covered by a test
- [ ] README.md, docs/design.md, docs/configuration.md, and the wait-for, ci-to-agent, and github-actions examples use --url; task ci passes
