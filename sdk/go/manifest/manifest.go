// Package manifest reads a module's module.yaml, the single source of truth for its registration facts (rule 51).
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

var (
	moduleIDPattern       = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
	permissionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
	apiPrefixPattern      = regexp.MustCompile(`^/api/([a-z][a-z0-9-]*)/v[1-9][0-9]*$`)
)

type Requirement struct {
	ID      string `yaml:"id" json:"id"`
	Version string `yaml:"version" json:"version"`
}

type Permission struct {
	Name         string   `yaml:"name" json:"name"`
	DefaultRoles []string `yaml:"defaultRoles" json:"defaultRoles"`
}

type Events struct {
	Publishes []string `yaml:"publishes"`
	Consumes  []string `yaml:"consumes"`
}

type ModuleManifest struct {
	ID          string        `yaml:"id"`
	Version     string        `yaml:"version"`
	Core        string        `yaml:"core"`
	APIPrefix   string        `yaml:"apiPrefix"`
	Requires    []Requirement `yaml:"requires"`
	Permissions []Permission  `yaml:"permissions"`
	Events      Events        `yaml:"events"`
}

// RegistrationRequest is the body of PUT /internal/v1/modules/{id} on core.
type RegistrationRequest struct {
	ID          string        `json:"id"`
	Version     string        `json:"version"`
	Core        string        `json:"core"`
	APIPrefix   string        `json:"apiPrefix"`
	Requires    []Requirement `json:"requires"`
	Permissions []Permission  `json:"permissions"`
}

func ParseManifest(data []byte) (ModuleManifest, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var parsed ModuleManifest
	if err := decoder.Decode(&parsed); err != nil {
		return ModuleManifest{}, fmt.Errorf("parse module.yaml: %w", err)
	}
	if err := parsed.Validate(); err != nil {
		return ModuleManifest{}, err
	}
	return parsed, nil
}

func (manifest ModuleManifest) Validate() error {
	var problems []string
	if !moduleIDPattern.MatchString(manifest.ID) {
		problems = append(problems, "id")
	}
	if _, err := semver.StrictNewVersion(manifest.Version); err != nil {
		problems = append(problems, "version")
	}
	if _, err := semver.NewConstraint(manifest.Core); err != nil {
		problems = append(problems, "core")
	}
	if match := apiPrefixPattern.FindStringSubmatch(manifest.APIPrefix); match == nil || match[1] != manifest.ID {
		problems = append(problems, "apiPrefix")
	}
	if !manifest.areRequirementsValid() {
		problems = append(problems, "requires")
	}
	if !manifest.arePermissionsValid() {
		problems = append(problems, "permissions")
	}
	if len(problems) > 0 {
		return errors.New("invalid module.yaml: " + strings.Join(problems, ", "))
	}
	return nil
}

func (manifest ModuleManifest) DeclaresPermission(name string) bool {
	for _, permission := range manifest.Permissions {
		if permission.Name == name {
			return true
		}
	}
	return false
}

func (manifest ModuleManifest) RegistrationRequest() RegistrationRequest {
	request := RegistrationRequest{
		ID:          manifest.ID,
		Version:     manifest.Version,
		Core:        manifest.Core,
		APIPrefix:   manifest.APIPrefix,
		Requires:    append([]Requirement{}, manifest.Requires...),
		Permissions: make([]Permission, 0, len(manifest.Permissions)),
	}
	for _, permission := range manifest.Permissions {
		request.Permissions = append(request.Permissions, Permission{
			Name: permission.Name, DefaultRoles: append([]string{}, permission.DefaultRoles...),
		})
	}
	return request
}

func (manifest ModuleManifest) areRequirementsValid() bool {
	seen := map[string]bool{}
	for _, requirement := range manifest.Requires {
		if !moduleIDPattern.MatchString(requirement.ID) || requirement.ID == manifest.ID || seen[requirement.ID] {
			return false
		}
		if _, err := semver.NewConstraint(requirement.Version); err != nil {
			return false
		}
		seen[requirement.ID] = true
	}
	return true
}

func (manifest ModuleManifest) arePermissionsValid() bool {
	seen := map[string]bool{}
	for _, permission := range manifest.Permissions {
		if !permissionNamePattern.MatchString(permission.Name) || !strings.HasPrefix(permission.Name, manifest.ID+".") || seen[permission.Name] {
			return false
		}
		seen[permission.Name] = true
	}
	return true
}
