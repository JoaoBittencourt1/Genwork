// Package versionfile reads the root VERSION file, the single owner of the platform version (rule 50).
package versionfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Read fails unless the file holds exactly one strict semantic version.
func Read(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	version := strings.TrimSpace(string(raw))
	if _, err := semver.StrictNewVersion(version); err != nil {
		return "", fmt.Errorf("%s holds %q, which is not a semantic version", path, version)
	}
	return version, nil
}

// Write replaces the file atomically, so a crash never leaves an installation without a version.
func Write(path, version string) error {
	if _, err := semver.StrictNewVersion(version); err != nil {
		return fmt.Errorf("write %s: %q is not a semantic version", path, version)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".VERSION-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.WriteString(version + "\n"); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
