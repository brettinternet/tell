package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Init creates only the selected project's template. Existing files are never
// rewritten, and symlinks/directories are refused rather than followed.
func Init(input Input, local bool) (path string, created bool, err error) {
	project, err := ProjectDir(input)
	if err != nil {
		return "", false, err
	}
	name, mode := "tell.yaml", os.FileMode(0o644)
	if local {
		name, mode = ".tell.yaml", 0o600
	}
	content := sharedTemplate
	if local {
		content = personalTemplate
	}
	return createTemplate(filepath.Join(project, name), mode, content)
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

const sharedTemplate = `# Tell settings. Inspection sends nothing; notify delivers via stderr or Apprise.
# Apprise is optional and external. Provider acceptance is not human acknowledgment.
# Routing belongs in user config or private .tell.yaml, never shared tell.yaml.
# Ignore .tell.yaml in Git. Keep credentials in your provider's configuration.
version: 1
# timeout: 10s
# apprise:
#   tag: project-alerts
#   format: markdown
`

const personalTemplate = `# Personal Tell settings; inspection sends nothing. notify uses stderr or Apprise.
# Apprise is optional and external. Provider acceptance is not human acknowledgment.
# Keep credentials in your provider's configuration. Ignore .tell.yaml in Git.
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
# hcom:
#   to: reviewer
`
