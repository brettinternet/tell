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

// InitUser creates personal configuration without reading project settings.
func InitUser(input Input) (path string, created bool, err error) {
	if !filepath.IsAbs(input.UserConfigDir) {
		return "", false, errors.New("user config directory must be absolute")
	}
	dir := filepath.Join(input.UserConfigDir, "tell")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, err
	}
	return createTemplate(filepath.Join(dir, "config.yaml"), 0o600, personalTemplate)
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

const sharedTemplate = `# Shared Tell settings. Commit this file to share project settings.
# Inspection sends nothing. notify routes: stderr, desktop, apprise, herdr, command.
# send transports: hcom, herdr. Acceptance is not acknowledgment.
# Run tell init --user to configure personal routing; do not set routes here.
# Keep credentials in your provider's configuration.
version: 1
# timeout: 10s
# apprise:
#   tag: project-alerts
#   format: markdown
`

const personalTemplate = `# Personal Tell settings. Set notify.via, then run tell doctor.
# Inspection sends nothing. notify routes: stderr, desktop, apprise, herdr, command.
# send transports: hcom, herdr. Acceptance is not acknowledgment.
# This user config is personal; --config/TELL_CONFIG selects another trusted file.
# Routing can also use TELL_* environment variables or flags where supported.
# Keep credentials in your provider's configuration.
version: 1
# timeout: 10s
# Personal routing examples:
# notify:
#   via: stderr
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
