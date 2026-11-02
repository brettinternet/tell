---
id: TELL-34
title: Add built-in shoutrrr notify route
status: done
type: feature
priority: medium
refs: ["https://github.com/nicholas-fedor/shoutrrr"]
created: 2026-11-02T03:15:00Z
updated: 2026-11-02T03:32:57Z
---

Why: Apprise is the only fan-out route to phone/chat services, and it needs a Python runtime and `apprise` on PATH. CI runners, containers, and minimal hosts often don't have either, so a cascade like `[apprise, desktop]` falls straight through to a route nobody sees. Shoutrrr (https://github.com/nicholas-fedor/shoutrrr) is a Go notification library covering most of the same services (Slack, Discord, Telegram, ntfy, Pushover, Gotify, Teams, SMTP, generic webhooks, and more). Linking it into Tell gives a built-in fan-out route that works wherever `tell` is installed, which makes it the natural fallback when Apprise is missing.

Decision (settled with the user): link the library in-process. Do not shell out to a `shoutrrr` CLI. Use the maintained fork `github.com/nicholas-fedor/shoutrrr` (v0.22.1 as of 2026-10, go 1.27, MIT), not `containrrr/shoutrrr`, whose last release was v0.8.0 in 2023. The fork's dependencies are light: paho MQTT, gorilla/websocket, mellium xmpp, x/oauth2. cobra/testify are its CLI/test-only deps and should not end up in Tell's binary.

Context:
- Route enums live in internal/config/config.go (top-level `notify.via` ~L676/690, listener routes ~L281), plus `tell.schema.json`/`tell.shared.schema.json`, the MCP `via` enum (internal/mcp/server.go ~L208), and `--via` help text (internal/cli/cli.go ~L47).
- Apprise is the template for a fan-out route: step parsing and `urls_env` (config.go ~L540-575), `notifyStepConfig` (internal/delivery/delivery.go), doctor readiness (internal/delivery/doctor.go), and fixed-hint diagnostics (internal/delivery/apprise.go). docs/design.md "Notify fallback cascades" and "Apprise delivery" describe the outcome rules this route must follow.
- Library API: `router.NewWithOptions(logger, types.SenderOptions{Timeout: ...}, urls...)` parses every URL and builds services with no network I/O. `(*ServiceRouter).SendContext(ctx, message, *types.Params) []error` returns one entry per service. A timeout is reported as `router.ErrServiceTimeout` or the caller's ctx error. Per-service failures are `*types.TargetError`. Set the title with `params.SetTitle`. Pass a discard `types.StdLogger`, and also check that the package-level root logger (`shoutrrr.SetLogger`) cannot write to stderr/stdout; the stderr route and `--json` output must stay clean.

Design:
- Config: `notify.via: shoutrrr` or a cascade step `- shoutrrr: {urls_env: NAME}`. Top-level `shoutrrr.urls_env` plus `--shoutrrr-urls-env` / `TELL_SHOUTRRR_URLS_ENV`, following the existing flag/env table. `urls_env` is required: Tell has no plain `url` key, because the URL is the credential. `shoutrrr.urls_env` is personal routing, so shared `tell.yaml` rejects it, like `notify.via`. NAME must be a valid environment variable name. Its value is one or more whitespace-separated shoutrrr URLs.
- Step label: `shoutrrr:<NAME>`. Never the URL or the scheme host.
- Delivery: read the env var at send time, build the router, and call `SendContext` with the step's budget context. Outcome mapping:
  - env unset or empty → `not_sent` / `missing_urls`
  - router construction (URL parse or unknown service) error → `not_sent` / `invalid_config`
  - every service error nil → `accepted`
  - any error that is a timeout or the caller's cancellation (`errors.Is` on `router.ErrServiceTimeout`, `context.DeadlineExceeded`, `context.Canceled`) → `uncertain`
  - any other error → `failed`, because with several URLs some may have been delivered (same rule as Apprise)
- Diagnostics: never include `err.Error()` or any library string in results, stderr, or JSON. Shoutrrr errors and logs can embed the URL, including tokens. Use fixed messages and reasons only, as in `appriseDiagnostic`. Classify by `errors.Is`/`errors.As` (timeout, a TLS `x509` error, `net.OpError`). Do not substring-match the text.
- Doctor: ready when the env var is set and non-empty and the router constructs. That check parses URLs without sending. Not-ready reasons say which of those failed, using the same fixed wording style as Apprise.
- MCP: add `shoutrrr` to the configured-route `via` enum only. No URL or env selector over MCP.
- Listener routes: add `shoutrrr` to the static `via` enum (config.go and the schema). No new per-action selector.

Scope / files: internal/config/config.go; internal/delivery/delivery.go, a new internal/delivery/shoutrrr.go, internal/delivery/doctor.go; internal/cli/cli.go and doctor.go; internal/mcp/server.go; tell.schema.json, tell.shared.schema.json; go.mod/go.sum; README.md, docs/design.md, docs/configuration.md, docs/listening.md (via enum); examples (below); tests beside the existing apprise/cascade tests.

Examples to update:
- examples/fallback/README.md: add a cascade where Apprise comes first and shoutrrr catches `apprise` `not_sent`/`missing_executable`, e.g. `[apprise: {tag: phone}, shoutrrr: {urls_env: PHONE_SHOUTRRR_URLS}, desktop]`. Explain that shoutrrr is built in, so it works where Apprise isn't installed, and that `failed` from a multi-URL step does not advance unless `fallback_on` lists it.
- examples/github-actions/: show shoutrrr as the no-install option. Set `TELL_NOTIFY_VIA: shoutrrr` and `TELL_SHOUTRRR_URLS_ENV: SHOUTRRR_URLS` with the URL from a secret, and drop the `pipx install apprise` step for that variant. Update its README.
- examples/user-config.yaml: add a commented shoutrrr step.
- examples/README.md: update the tables that list requirements (the apprise, github-actions, and fallback rows), and add shoutrrr where relevant.
- README.md route table: `shoutrrr | nothing (built in); URLs in an env var you name`.
- docs/design.md: add a "Shoutrrr delivery" section with the outcome table, the no-library-text rule, and the pinned module version, plus mention shoutrrr in "Commands" and the cascade docs.

Non-goals: shelling out to a shoutrrr binary; mapping Tell's `--type` to shoutrrr levels or service-specific params; per-service query-param templating; a plain `url` config key or URLs in shared config; MCP URL/env selectors; changing the default route or the default cascade; retries; `send` (agent) support.

Dependencies: none.

Implementer notes: record the `bin/tell` size before and after linking, and put both numbers in the summary. If the growth is large (more than ~5 MB), say so before merging. Check `go mod why` for anything unexpected and run `task ci`, which includes vuln and license checks if configured. For tests, use a local `httptest` server with a generic webhook URL (`generic+http://127.0.0.1:PORT/hook` or the fork's equivalent) rather than mocks.

## Acceptance criteria

- [x] `notify.via: shoutrrr` and cascade step `shoutrrr: {urls_env: NAME}` deliver in-process via linked github.com/nicholas-fedor/shoutrrr; no external executable needed. `shoutrrr.urls_env` / `--shoutrrr-urls-env` / `TELL_SHOUTRRR_URLS_ENV` work, shared tell.yaml rejects it, and config/schemas/MCP/listener via enums include shoutrrr.
  Evidence: 5ede1f7: internal/delivery/shoutrrr.go links nicholas-fedor/shoutrrr v0.22.1; TestShoutrrrOutcomes/TestShoutrrrURLsEnvIsPersonal pass; manual TELL_SHOUTRRR_URLS_ENV shows origin in tell config; enums updated in config, schemas, MCP, listener
- [x] Outcomes: unset/empty env → not_sent/missing_urls; invalid URL or unknown service → not_sent/invalid_config; all ok → accepted; timeout or cancel → uncertain; other send errors → failed. httptest-backed tests (generic webhook, no mocks) cover accepted, server error → failed, slow server → uncertain, invalid URL → not_sent, and a cascade where a missing apprise executable falls back to shoutrrr.
  Evidence: go test ./internal/cli -run Shoutrrr: accepted, server error→failed, slow→uncertain, conn refused→failed, invalid/unknown→not_sent invalid_config, empty→missing_urls; TestShoutrrrCascadeAfterMissingApprise passes (httptest generic webhook)
- [x] Tests prove the URL value (including a token) never appears in --json output, stderr, doctor output, or attempt labels, and that library logging writes nothing to stdout/stderr.
  Evidence: assertNoShoutrrrLeak on json/human/doctor/labels in cli tests; integration TestShoutrrrBinaryStreams runs real bin/tell: one JSON line, empty stderr, no token; mutations (stderr logger, err text) were caught
- [x] `tell doctor --via shoutrrr` reports ready/not-ready (env missing, URL parse failure) without sending.
  Evidence: TestShoutrrrDoctor: ready, missing_urls, invalid_config; webhook call count 0
- [x] README route table, docs/design.md (Shoutrrr delivery section), docs/configuration.md, docs/listening.md, and examples (fallback cascade, github-actions no-install variant, user-config.yaml, examples/README.md tables) are updated. Binary size before/after is recorded in the summary. `task ci` passes.
  Evidence: Docs + examples updated in 5ede1f7 (fallback cascade, github-actions/notify-on-failure-shoutrrr.yaml, user-config.yaml, examples/README.md). bin/tell 26,531,986 → 30,663,218 bytes (+4.13 MB). task ci passed (govulncheck, check, test, race, smoke)

## Summary

Built-in shoutrrr notify route (single and cascade step) via linked nicholas-fedor/shoutrrr v0.22.1, URLs read from a named env var and never surfaced; fixed diagnostic hints only. Binary size: 26,531,986 → 30,663,218 bytes (+4.13 MB, ~15.6%). Commit 5ede1f7 fast-forwarded to main; worktree removed. Self-review: no further defects; global stdlib log discarded because library services log config text.
