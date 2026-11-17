---
id: TELL-51
title: Ship Tell.app in macOS release archives
status: todo
type: feature
priority: high
labels: [desktop]
created: 2026-11-17T20:40:05Z
updated: 2026-11-17T20:40:05Z
---

## Context

Tell.app (`macos/TellNotifier`) is only installed by `task macos:install` from a source checkout. Release archives contain only `bin/tell`, so mise, Homebrew, and tarball users get the osascript fallback: banners attributed to Script Editor, no click actions (TELL-43), and `--url` degraded to a body line (TELL-45).

Gatekeeper only assesses quarantined files. mise's GitHub backend, Homebrew formulae (not casks), and `curl` downloads do not set `com.apple.quarantine`, so an ad-hoc signed app delivered that way runs without a Developer ID. Browser downloads are quarantined. This is the zero-cost path; a Developer ID and notarization stay a separate, later decision.

## Scope

- Build Tell.app as a universal (arm64 + x86_64) ad-hoc signed bundle on a macOS release job and include it in both macOS archives as `Tell.app` beside `bin/`. The release currently packages on `ubuntu-latest`, so add or move the macOS packaging step; keep Linux archives unchanged.
- `desktopHelper` looks for the helper in this order: `~/Applications/Tell.app` (existing, source installs win), then `Tell.app` beside the resolved `tell` executable (`<archive root>/Tell.app`, following symlinks such as mise and Homebrew shims). Keep the existing executable/regular-file checks.
- First use from a release install must register the bundle with Launch Services and request permission without a separate manual step, or `tell doctor --via desktop` must state the exact one-time step. Never start a service.
- Update the Homebrew tap formula (separate repository) to install the bundle alongside the binary, only if it does not do so automatically.
- Document installation per channel in README.md and docs/design.md#desktop-delivery, including the quarantine caveat for browser downloads.

## Non-goals

Developer ID signing, notarization, a Homebrew cask, auto-update, Linux or Windows helpers, changing the helper's protocol.

## Dependencies

None. Related: TELL-43 (click actions), TELL-45 (`--url`).

## Modified-file boundaries

.taskfiles/macos.yaml, scripts/package.sh, .github/workflows/release.yaml, internal/delivery (desktop.go, doctor.go and their tests), README.md, docs/design.md, docs/development.md.

## Verification

Package a release locally on macOS; install it with mise's GitHub backend from a test release or local archive on a Mac without `~/Applications/Tell.app`; confirm `xattr` shows no quarantine, `tell doctor --via desktop` reports `tell-app`, and a `--url` notification click opens the browser after `tell` exits. Repeat after upgrading to a second version to confirm the click still reaches a present bundle. `task ci`.

## Acceptance criteria

- [ ] macOS release archives contain a universal ad-hoc signed Tell.app beside bin/tell; Linux archives are unchanged; built by the release workflow on a macOS runner
- [ ] tell uses ~/Applications/Tell.app first, then Tell.app beside the resolved tell executable (mise and Homebrew symlinks followed); covered by a test
- [ ] On a Mac without ~/Applications/Tell.app, a mise-installed release shows no quarantine attribute, tell doctor --via desktop reports tell-app, and clicking a --url notification after tell exits opens the browser; still works after upgrading to a second version; recorded
- [ ] README.md and docs/design.md describe Tell.app per install channel, including the browser-download quarantine caveat; task ci passes
