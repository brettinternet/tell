---
id: TELL-51
title: Ship signed and notarized Tell.app in macOS releases
status: in-progress
type: feature
priority: high
labels: [desktop]
created: 2026-11-17T20:40:05Z
updated: 2026-11-17T20:47:29Z
---

## Context

Tell.app (`macos/TellNotifier`) is only installed by `task macos:install` from a source checkout. Release archives contain only `bin/tell`, so mise, Homebrew, and tarball users get the osascript fallback: banners attributed to Script Editor, no click actions (TELL-43), and `--url` degraded to a body line (TELL-45).

The maintainer has an Apple Developer ID. Signing and notarizing make the app and binary pass Gatekeeper on every channel, including quarantined browser downloads, so no per-channel caveat is needed. Follow the release signing pattern already used by ../HyperWindow (`.github/workflows/ci.yaml`, `scripts/package-release.sh`) and ../DisplayPanel (`scripts/import-signing.py`, `scripts/notarize.sh`, the credential-check job).

## Scope

- Build Tell.app as a universal (arm64 + x86_64) bundle on a macOS release job. Sign it with `Developer ID Application: Brett Gardiner (4NQM9EVPZ3)`, hardened runtime (`--options runtime`), and a secure timestamp; notarize and staple it. Include it in both macOS archives as `Tell.app` beside `bin/`.
- Build the darwin `tell` binaries on the same macOS job, sign them with the same identity, hardened runtime, and timestamp, and notarize them (a bare Mach-O cannot be stapled; first run checks online). Linux archives stay unchanged and keep building as today.
- Release job uses the protected GitHub `release` environment with secrets `DEVELOPER_ID_P12_BASE64`, `DEVELOPER_ID_P12_PASSWORD`, `NOTARY_KEY_P8_BASE64`, `NOTARY_KEY_ID`, `NOTARY_ISSUER_ID`; the identity is plain workflow env (`DEVELOPER_ID_IDENTITY`), as in HyperWindow. Import the certificate into a temporary keychain with a random password, never add trust, and confirm the identity with `security find-identity -v -p codesigning`. Write the notary key with `umask 077` under `$RUNNER_TEMP` and remove it in an `always()` step, as in DisplayPanel.
- Packaging fails closed: verify with `codesign --verify --deep --strict`, require `spctl --assess` to report `source=Notarized Developer ID` for the app, validate the staple, and print `notarytool log` on rejection. Never publish an unsigned or un-notarized macOS archive. Run the signing/notary credential check (no submission) before tagging so a missing or expired secret fails before a release is cut.
- `task macos:install` stays ad-hoc signed for source checkouts; no local credentials are required to build or test.
- `desktopHelper` looks for the helper in this order: `~/Applications/Tell.app` (existing, source installs win), then `Tell.app` beside the resolved `tell` executable (`<archive root>/Tell.app`, following symlinks such as mise and Homebrew shims). Keep the existing executable/regular-file checks.
- First use from a release install must register the bundle with Launch Services and request notification permission without a separate manual step, or `tell doctor --via desktop` must state the exact one-time step. Never start a service.
- Update the Homebrew tap formula (separate repository) to install the bundle alongside the binary, only if it does not do so automatically.
- Document installation in README.md and docs/design.md#desktop-delivery, and the release secrets and signing flow in docs/development.md.

## Non-goals

A Homebrew cask, auto-update, a `.pkg` or DMG, Linux or Windows helpers, signing source-checkout builds with the Developer ID, changing the helper's protocol.

## Human-required

The maintainer creates the `release` environment and adds the five secrets (base64 Developer ID Application `.p12` and its password, base64 App Store Connect API `.p8`, key ID, issuer ID). The same values HyperWindow uses can be reused. Implementation that does not need them proceeds first; prompt for them when the release job is ready to verify.

## Dependencies

None. Related: TELL-43 (click actions), TELL-45 (`--url`), TELL-52 (Linux equivalent).

## Modified-file boundaries

.taskfiles/macos.yaml, .taskfiles (cli/release), scripts/package.sh and new signing/notarization scripts under scripts/, .github/workflows/release.yaml, macos/TellNotifier (Info.plist or entitlements only if hardened runtime needs them), internal/delivery (desktop.go, doctor.go and their tests), README.md, docs/design.md, docs/development.md.

## Verification

Cut a test release through the workflow. Download the macOS archive with a browser so it is quarantined, extract it, and confirm `spctl --assess --type execute -vv Tell.app` reports `source=Notarized Developer ID`, `xcrun stapler validate Tell.app` succeeds, and `bin/tell` runs without a Gatekeeper prompt. On a Mac without `~/Applications/Tell.app`, install the release with mise; confirm `tell doctor --via desktop` reports `tell-app` and clicking a `--url` notification after `tell` exits opens the browser. Repeat after upgrading to a second version. `task ci`.

## Acceptance criteria

- [ ] macOS release archives contain a universal Tell.app beside bin/tell, both Developer ID signed with hardened runtime and notarized (app stapled); built on a macOS release job using the protected release environment; packaging fails closed on unsigned or un-notarized output; Linux archives unchanged
- [ ] tell uses ~/Applications/Tell.app first, then Tell.app beside the resolved tell executable (mise and Homebrew symlinks followed); covered by a test
- [ ] A browser-downloaded (quarantined) macOS archive passes spctl as Notarized Developer ID and runs bin/tell without a prompt; on a Mac without ~/Applications/Tell.app a mise-installed release reports tell-app in tell doctor --via desktop and a --url click after tell exits opens the browser, still after upgrading; recorded
- [ ] README.md and docs/design.md describe Tell.app installation; docs/development.md documents the release secrets and signing flow; task macos:install still works ad-hoc without credentials; task ci passes
