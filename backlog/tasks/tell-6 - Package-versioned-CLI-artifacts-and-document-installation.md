---
id: TELL-6
title: Package versioned CLI artifacts and document installation
status: Done
assignee:
  - '@pi'
created_date: '2026-10-16 20:29'
updated_date: '2026-10-16 23:01'
labels: []
dependencies:
  - TELL-4
references:
  - ../hum/.github/workflows/release.yaml
  - ../homebrew-tap/Formula/worklease.rb
  - ../worklease/.github/workflows/release.yml
modified_files:
  - .github/workflows/release.yaml
  - .taskfiles/cli.yaml
  - Taskfile.dist.yaml
  - scripts/
  - README.md
  - docs/development.md
  - docs/design.md
type: chore
ordinal: 6000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Why: once Tell can deliver notifications, consumers need reproducible binaries rather than building the repository on each machine. The first release also turns today's free-to-change interfaces into a compatibility contract.

Context:
- cmd/tell declares buildVersion, buildCommit, and buildTime for -X linker injection, but task cli:build injects nothing, so tell --version reports dev and unknown.
- ../hum/.github/workflows/release.yaml is the reference: tag-triggered, verifies the tag commit's main-branch CI run, builds CGO_ENABLED=0 -trimpath archives for linux and darwin on amd64 and arm64, and publishes checksums.txt.
- worklease names archives worklease-vX.Y.Z-OS-ARCH.tar.gz (OS macos or linux, ARCH arm64 or x64) containing bin/worklease; hum omits the v prefix and ships a flat archive. Follow worklease. The tap's scheduled workflow (../homebrew-tap) pulls new releases into formulas, so Tell's CI never touches the tap; adding the formula is a follow-up task. mise's github backend can install the same archives.
- Compatibility surfaces: config version 1, JSON schema_version 1, and the exit codes in docs/design.md. This task waits for TELL-4 because TELL-4 replaces the bootstrap hcom.to setting (TELL_HCOM_TO, --to); releasing first would break the config version 1 promise.
- CI (.github/workflows/ci.yaml) runs task ci on Linux and macOS with SHA-pinned Actions.

Scope: Taskfile/.taskfiles packaging tasks, a release workflow, checksums, installation, version, and compatibility docs.

Non-goals: Windows, daemons, updating the Homebrew tap from CI, changelog pushes from CI, tag pushes or release publication while implementing this task.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 A local packaging task produces Linux and macOS arm64/x64 archives named and laid out like worklease (tell-vX.Y.Z-OS-ARCH.tar.gz containing bin/tell) with linker-injected version, commit, and build time plus a checksum file that verifies; the host artifact's --version reports the injected metadata.
- [x] #2 A release workflow verifies the exact tag commit passed CI before publishing, with minimal permissions and SHA-pinned Actions; no release is published during verification.
- [x] #3 Docs cover installation (release archive, mise github backend, and the brettinternet tap as a pending follow-up), optional external-provider prerequisites, and the compatibility promise for config version 1, JSON schema_version 1, and exit codes; task ci and the local packaging checks pass.
- [x] #4 Version-tag pushes publish the platform archives and checksums.txt to brettinternet/tell GitHub Releases (not only Actions artifacts), with release notes; adapt the relevant hum or worklease release workflow without importing their unrelated container, Windows, or changelog-push features.
- [x] #5 The release workflow includes clean-environment mise installation smoke checks on Linux and macOS, x64 and arm64: mise exec github:brettinternet/tell@latest -- tell --version resolves the expected release using default asset selection, without asset_pattern, matching overrides, custom URLs, or a custom plugin. Documentation shows this plain GitHub-backend command. Before publication is authorized, validate archive layout and workflow checks locally; record the live install check as pending, never claim it ran or publish a release solely for verification.
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 Acceptance evidence is recorded in notes, relevant checks pass, and intended changes are committed without an unauthorized push.
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Add shared local four-platform packaging and archive/metadata/checksum validation tasks using the existing linker fields.
2. Adapt the hum CI gate and GitHub release publication; add isolated four-platform default mise installation checks after publication.
3. Document installation, provider prerequisites, compatibility, and release verification. Run local packaging, task ci, workflow lint, and one independent release-safety review.
4. Commit implementation, merge locally, record evidence (live release installation pending by design), finalize task and remove the owned worktree.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Implemented four-platform packaging and tag-release workflow in .worktrees/tell-6-release. Verification: task ci passed (gofmt, vet, Staticcheck, unit/race/smoke tests, Gitleaks and reachable vulnerability scan); VERSION=v1.2.3 task cli:package passed with default metadata and again with BUILD_TIME=2026-10-16T00:00:00Z. All four SHA-256 checks and exact bin/tell layouts passed; macOS arm64 extracted binary reported version 1.2.3, full source commit 61a03033fe39725756f8884e3790da536565aacf and injected timestamp. file identified Linux x86-64/aarch64 static ELF and macOS x86_64/arm64 Mach-O binaries. Invalid v1x2x3 version rejected. mise exec actionlint@1.7.12 shellcheck@0.11.0 -- actionlint .github/workflows/release.yaml and shellcheck of both packaging scripts passed. YAML LSP returned unknown (not used as acceptance evidence). Independent release-safety review pending. No push, tag or release publication; four-platform LIVE mise installation remains pending until an explicitly authorized release, as AC5 requires.

Independent read-only reviewer completed one general review: no validated findings. Implementation committed as d0b47a0 (chore: package versioned release artifacts), after task check:staged passed. Integration pending: concurrent primary-checkout documentation edits overlap README.md and docs/design.md. User explicitly chose to wait for the other writer to commit; do not stash, overwrite, or commit their changes. Next: verify the Worklease claim, refresh primary status, merge the committed documentation into tell-6-release and reconcile only task-scoped docs if needed, rerun affected checks, merge locally to main, finalize Backlog and clean the receipt-owned worktree. Live mise install remains pending by AC5; no publication is authorized.

Integration complete: waited for independent documentation commit 508d1df, merged it into the implementation branch and preserved its rewritten structure while retaining release instructions and compatibility promises. Reconciliation commit 97449c1; reran task ci and VERSION=v1.2.3 task cli:package successfully on that commit (host --version reported full 97449c1c2115664dc01e69b66140a308b5befc7f metadata). main fast-forwarded to 97449c1. Worktrunk removed the receipt-verified tell-6-release checkout and branch; associated wE8 workspace was inspected (idle zsh only) and removed by the hook. AC2/4 workflow verified by Actionlint, ShellCheck and independent release-safety review; publication path creates GitHub Releases with four archives, checksums and generated notes only after exact-tag main-push CI success. AC5 is satisfied at its explicitly allowed pre-publication boundary: matrix and default mise command validated statically, live installation PENDING until authorized publication. No push or release occurred. Unrelated TELL-14 task left untouched. No implementation blocker or next implementation step remains.
<!-- SECTION:NOTES:END -->

## Final Summary

<!-- SECTION:FINAL_SUMMARY:BEGIN -->
Implemented local four-platform release packaging, SHA-256/layout/host-version verification, exact-tag main-CI-gated GitHub Releases with generated notes, and clean default-mise installation checks on all four platforms. Documented installation, optional providers, version metadata and compatibility contracts. Verified task ci, packaging (default and explicit metadata), invalid version rejection, Actionlint/ShellCheck and independent review (no findings). Commits d0b47a0 and 97449c1 merged to main; owned worktree/branch/workspace removed. Live mise install remains pending authorized publication as required by AC5; nothing pushed or published.
<!-- SECTION:FINAL_SUMMARY:END -->
