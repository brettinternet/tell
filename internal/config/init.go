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
	path = filepath.Join(project, name)
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
	_, writeErr := io.WriteString(file, template)
	return path, true, errors.Join(writeErr, file.Close())
}

const template = `# Tell settings. This bootstrap only inspects configuration; it sends nothing.
# Ignore .tell.yaml in Git. Keep credentials in your provider's configuration.
version: 1
# timeout: 10s
# notify:
#   via: stdout
# apprise:
#   config: /absolute/path/to/apprise.yaml
#   tag: project-alerts
# herdr:
#   session: main
# hcom:
#   to: reviewer
`
