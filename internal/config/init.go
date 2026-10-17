package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Init creates the selected project's shared template. Existing files are never
// rewritten, and symlinks/directories are refused rather than followed.
func Init(input Input) (path string, created bool, err error) {
	project, err := ProjectDir(input)
	if err != nil {
		return "", false, err
	}
	return createTemplate(filepath.Join(project, "tell.yaml"), 0o644, sharedTemplate)
}

// InitLocal creates repository-personal configuration in the project's common
// Git directory, which Git never commits and linked worktrees share.
func InitLocal(input Input) (path string, created bool, err error) {
	project, err := ProjectDir(input)
	if err != nil {
		return "", false, err
	}
	dir, err := gitCommonDir(project)
	if err != nil {
		return "", false, err
	}
	if dir == "" {
		return "", false, fmt.Errorf("%s is not a Git checkout; use tell init --user for personal config", project)
	}
	return createTemplate(filepath.Join(dir, "tell.yaml"), 0o600, localHeader+personalBody)
}

// InitUser creates personal configuration without reading project settings.
func InitUser(input Input) (path string, created bool, err error) {
	if !filepath.IsAbs(input.UserConfigDir) {
		return "", false, errors.New("user config directory must be absolute")
	}
	dir := filepath.Join(input.UserConfigDir, "tell")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, err
	}
	return createTemplate(filepath.Join(dir, "config.yaml"), 0o600, userHeader+personalBody)
}

func createTemplate(path string, mode os.FileMode, content string) (string, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return path, false, statErr
		}
		if !info.Mode().IsRegular() {
			return path, false, fmt.Errorf("%s: existing path is not a regular file", path)
		}
		return path, false, nil
	}
	if err != nil {
		return path, false, err
	}
	_, writeErr := io.WriteString(file, content)
	return path, true, errors.Join(writeErr, file.Close())
}

const sharedTemplate = `# yaml-language-server: $schema=https://raw.githubusercontent.com/brettinternet/tell/main/tell.shared.schema.json
# Shared Tell settings. Commit this file to share project settings.
# Inspection sends nothing. notify routes: stderr, desktop, apprise, herdr, command.
# send transports: hcom, herdr. Acceptance is not acknowledgment.
# Routes are personal, not set here: tell init --user (all projects) or --local (this repo).
# Keep credentials in your provider's configuration.
version: 1
# timeout: 10s
# apprise:
#   tag: project-alerts
#   format: markdown
`

const userHeader = `# yaml-language-server: $schema=https://raw.githubusercontent.com/brettinternet/tell/main/tell.schema.json
# Personal Tell settings for every project. Set notify.via, then run tell doctor.
`

const localHeader = `# yaml-language-server: $schema=https://raw.githubusercontent.com/brettinternet/tell/main/tell.schema.json
# Personal Tell settings for this repository and its worktrees; overrides user config.
# Git never commits files inside .git. Set notify.via, then run tell doctor.
`

const personalBody = `# Inspection sends nothing. notify routes: stderr, desktop, apprise, herdr, command.
# send transports: hcom, herdr. Acceptance is not acknowledgment.
# --config/TELL_CONFIG selects another trusted file instead of automatic files.
# Routing can also use TELL_* environment variables or flags where supported.
# Keep credentials in your provider's configuration.
version: 1
# timeout: 10s
# Personal routing examples:
# notify:
#   via: stderr
#   command: [/absolute/path/to/notifier]   # required when via: command
# apprise:
#   config: /absolute/path/to/apprise.yaml
#   tag: project-alerts
#   format: markdown
# herdr:
#   session: main
# send:
#   via: hcom
#   to: reviewer
#   from: tell-automation
# MCP route/recipient overrides are disabled for agents by default.
# mcp:
#   agent_targets: false
`
