---
id: TELL-13
title: Add Tell to the brettinternet Homebrew tap
status: Done
assignee: []
created_date: '2026-10-16 22:25'
updated_date: '2026-10-17 00:00'
labels:
  - reviewed
dependencies:
  - TELL-6
references:
  - ../homebrew-tap/Formula/worklease.rb
  - ../homebrew-tap/.github/workflows/update-formulae.yml
  - ../homebrew-tap/.github/scripts/update-formula.rb
  - ../homebrew-tap/.github/scripts/check-homebrew-tap.sh
documentation:
  - ../homebrew-tap/README.md
type: chore
ordinal: 7000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: hum and worklease install with brew install brettinternet/tap/NAME and stay current automatically. Tell should install and upgrade the same way on the machines where scripts, scheduler hooks, and agents call it. The tap lives in ../homebrew-tap (github.com/brettinternet/homebrew-tap), not this repository.

Context:
- Formula/worklease.rb is the template: on_macos/on_linux and on_arm/on_intel blocks with release URLs and sha256 values, bin.install "bin/worklease", and a version test. Formula/hum.rb shows a flat, macOS-only archive.
- .github/workflows/update-formulae.yml runs every 6 hours, loops over "hum worklease", downloads each latest release's checksums.txt, and runs .github/scripts/update-formula.rb. That script's formulae map records each repository, asset version prefix ("" for hum, "v" for worklease), and platforms. Tell needs an entry in both, or its formula never updates.
- .github/scripts/check-homebrew-tap.sh OWNER/TAP FORMULA... symlinks the checkout as a temporary tap, then runs brew style, brew audit --strict --online, brew install, and brew test. It refuses to run when that tap is already installed, so use an unused tap name.
- Archive names, layout, and platforms come from TELL-6. tell --version prints "tell version X.Y.Z (commit ..., built ...)".
- No homebrew-core formula or cask is named tell (checked 2026-10-16).
- The tap has no AGENTS.md or backlog; follow its existing commit style (for example "update formulae: ...").
- This needs a published Tell release. Publishing a release (pushing a tag) and pushing the tap are human actions: do neither without explicit instruction. If no release exists, record the blocker and stop.

Scope: ../homebrew-tap Formula/tell.rb, the update-formula.rb entry, the update-formulae.yml loop, and the tap README; Tell's README and installation docs.

Non-goals: homebrew-core submission, a cask, a man page, bottles or source builds, changing Tell's release workflow, pushing either repository.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 ../homebrew-tap/Formula/tell.rb installs the latest published Tell release for every platform TELL-6 publishes, using checksums from that release's checksums.txt, and its test asserts that the formula version appears in tell --version.
- [x] #2 From ../homebrew-tap, .github/scripts/check-homebrew-tap.sh tell-check/tap tell exits 0 (style, strict online audit, install, test); afterwards brew uninstall tell leaves no installed tell keg.
- [x] #3 update-formulae.yml includes tell, and running ruby .github/scripts/update-formula.rb tell VERSION CHECKSUMS against the current release's checksums.txt leaves Formula/tell.rb unchanged.
- [x] #4 The tap README lists Tell with brew trust and brew install commands; Tell README shows brew install brettinternet/tap/tell without a pending caveat; each repository is committed and merged. Release and necessary pushes are authorized by the user (2026-10-16), superseding the original no-push restriction.
<!-- AC:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Publish authorized v0.1.0 after local CI, packaging, and workflow lint. 2. Add four-platform formula, updater entry and workflow loop, plus installation docs. 3. Verify published checksums, updater idempotence, Homebrew style/audit/install/test/uninstall, and release installation matrix. 4. Commit, merge, publish as authorized, remove owned worktrees, and release claim.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
User authorized creating the first release and pushing as needed to unblock this task. v0.1.0 targets 99d87d8; local task ci, VERSION=v0.1.0 task cli:package, and actionlint with shellcheck passed. Pushed exact commit to main and v0.1.0 tag; release run 38982820565. Implementation worktrees: tell/.worktrees/tell-13-homebrew and homebrew-tap/.worktrees/tell-13-homebrew.

Verification: task ci and VERSION=v0.1.0 task cli:package passed; actionlint with shellcheck passed for release and tap workflows. Release run 38982820565 succeeded, including live mise installs on macOS Intel/ARM and Linux Intel/ARM. Downloaded all four published archives; shasum -a 256 -c checksums.txt passed. Formula updater 0.1.0 preserved formula SHA256 c9400661d73a9eea3c90e4355888780b9c89f7ea063c462cb313d852361f3fed. Homebrew check-homebrew-tap.sh tell-check/tap tell passed style, strict online audit, install and version test, including a rerun after merging upstream tap updates; brew uninstall tell succeeded and /opt/homebrew/Cellar/tell is absent. Initial install exposed single-root archive stripping; formula correctly installs tell from the stripped root. One scoped diff review found no remaining defects. Ruby syntax and staged secret scans passed; LSP unavailable outside the Tell workspace. Tell docs commit 604e215 and tap formula commit f8ce1a5 (integration bfbbe1a) merged to main. User explicitly overrode the original no-push restriction: authorized release and pushes as needed; v0.1.0, Tell main and tap main were pushed. Both session-owned worktrees and branches removed with Worktrunk; their exact Herdr workspaces wEG and wEK are gone. No owned worktrees retained; no blocker or next step remains. Homebrew itself auto-installed style-check dependencies, updated to 7.0.9, and auto-removed unused openssl@3 during the first uninstall; no config files were removed manually.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Published v0.1.0 and added the four-platform Tell Homebrew formula, automatic release updates, and installation docs. Verified local CI/package/lint, all four live mise installs, published checksums, updater idempotence, and Homebrew style/audit/install/test/uninstall. Merged and pushed authorized changes (Tell 604e215; tap f8ce1a5 and bfbbe1a). Owned worktrees, branches and linked workspaces cleaned up.
<!-- SECTION:FINAL_SUMMARY:END -->
