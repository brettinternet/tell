# Changelog

Notable changes to tell are documented here. This file is generated from conventional commits by [git-cliff](https://git-cliff.org/).

## Unreleased

### Documentation

- Draft shared routing trust proposal
- Close TELL-25 after release verification

## [v0.3.0](https://github.com/brettinternet/tell/compare/v0.2.1...v0.3.0) - 2026-10-17

### Added

- **cli:** Add restrained terminal styling
- Publish configuration schemas

### Fixed

- Add safe Apprise failure diagnostics
- **cli:** Simplify version output and add version command

### Documentation

- Add schema hint to example config

## [v0.2.0](https://github.com/brettinternet/tell/compare/v0.1.0...v0.2.0) - 2026-10-17

### Added

- **config:** Load repository-personal config from .git/tell.yaml
- **cli:** Simplify orientation and config output
- **cli:** Accept multi-word messages
- **cli:** Scope routing flags to commands
- **cli:** Add short flags for common options

### Fixed

- Show unquoted enum defaults and notify.command template example
- Parse flags after message dash and update examples for scoped flags
- Tailor init templates and next steps
- **cli:** Explain delivery and readiness failures
- **cli:** Report actionable usage and configuration errors
- Stop loading repository-local personal routing
- **init:** Ignore ambient TELL\_\* environment

### Documentation

- **cli:** Clarify help and shell completion setup
- Add examples library for notifications, agents, and automation
- Document released Homebrew installation

## [v0.1.0](https://github.com/brettinternet/tell/commits/v0.1.0) - 2026-10-16

### Added

- Expose delivery tools over MCP stdio
- Send messages to existing agents
- Check route readiness without sending
- Deliver notifications to explicit Herdr sessions
- Deliver notifications through personal commands
- Deliver native desktop notifications
- Redact secrets before notification delivery
- Integrate notification normalization
- Normalize outbound notification text
- Deliver stderr and Apprise notifications
- Keep delivery routing in personal configuration
- Add layered configuration CLI

### Fixed

- **send:** Report interrupted recipient checks as not sent
- Redact detected secrets across title and body
- **cli:** Allow --config without HOME
- **config:** Accept comment-only files as empty layers

### Documentation

- Rewrite docs to be concise and example-led
- Add verified Noon transition hook
- Plan Homebrew tap formula
- Release after send and refresh route tasks
- Complete TELL-10 verification
- Complete TELL-9 verification
- Keep Tell scheduler-neutral and stateless
- Complete TELL-2 verification
- Complete TELL-7 verification
- Skip missed scheduled agent instructions
- Plan scheduled agent instructions and release contract
- Plan message safety, custom routes, and agent send
- Define delivery outcomes and route ownership
