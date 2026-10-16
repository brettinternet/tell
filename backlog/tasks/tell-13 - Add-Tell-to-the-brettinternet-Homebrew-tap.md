---
id: TELL-13
title: Add Tell to the brettinternet Homebrew tap
status: To Do
assignee: []
created_date: '2026-10-16 22:25'
labels: []
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
- [ ] #1 ../homebrew-tap/Formula/tell.rb installs the latest published Tell release for every platform TELL-6 publishes, using checksums from that release's checksums.txt, and its test asserts that the formula version appears in tell --version.
- [ ] #2 From ../homebrew-tap, .github/scripts/check-homebrew-tap.sh tell-check/tap tell exits 0 (style, strict online audit, install, test); afterwards brew uninstall tell leaves no installed tell keg.
- [ ] #3 update-formulae.yml includes tell, and running ruby .github/scripts/update-formula.rb tell VERSION CHECKSUMS against the current release's checksums.txt leaves Formula/tell.rb unchanged.
- [ ] #4 The tap README lists Tell with brew trust and brew install commands; Tell's README installation section shows brew install brettinternet/tap/tell without a pending caveat; each repository's changes are committed and nothing is pushed.
<!-- AC:END -->
