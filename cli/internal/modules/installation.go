package modules

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

const installationHeader = "# Written by genwork. Choose modules with `genwork modules`.\n"

// InstallationConfig is the installation's module choice. Chosen is false until modules were picked once, even if none was.
type InstallationConfig struct {
	Chosen  bool
	Modules []string
}

type installationFile struct {
	Modules []string `yaml:"modules"`
}

func ReadInstallationConfig(path string) (InstallationConfig, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return InstallationConfig{}, nil
	}
	if err != nil {
		return InstallationConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var file installationFile
	if err := decoder.Decode(&file); err != nil && !errors.Is(err, io.EOF) {
		return InstallationConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}
	installed := slices.Clone(file.Modules)
	slices.Sort(installed)
	return InstallationConfig{Chosen: true, Modules: slices.Compact(installed)}, nil
}

// WriteInstallationConfig replaces the file atomically, so an interrupted write never leaves half a choice behind.
func WriteInstallationConfig(path string, installed []string) error {
	sorted := slices.Clone(installed)
	slices.Sort(sorted)
	body, err := yaml.Marshal(installationFile{Modules: append([]string{}, slices.Compact(sorted)...)})
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".genwork-*.yaml")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.WriteString(installationHeader + string(body)); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (config InstallationConfig) IsInstalled(id string) bool {
	return slices.Contains(config.Modules, id)
}
