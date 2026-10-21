package modules

import (
	"regexp"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/Masterminds/semver/v3"
)

var (
	moduleNamePattern     = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)
	permissionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)
	apiPrefixPattern      = regexp.MustCompile(`^/api/([a-z][a-z0-9-]*)/v[1-9][0-9]*$`)
)

type RequirementRequest struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type PermissionRequest struct {
	Name         string   `json:"name"`
	DefaultRoles []string `json:"defaultRoles"`
}

// RegisterModuleRequest carries the registration facts of a module's module.yaml (rule 51).
type RegisterModuleRequest struct {
	ID          string               `json:"id"`
	Version     string               `json:"version"`
	Core        string               `json:"core"`
	APIPrefix   string               `json:"apiPrefix"`
	Requires    []RequirementRequest `json:"requires"`
	Permissions []PermissionRequest  `json:"permissions"`
}

// ToRegistration validates the request against the module name taken from the path.
func (request RegisterModuleRequest) ToRegistration(pathModuleName string) (Registration, error) {
	fields := map[string]string{}
	if request.ID != pathModuleName || !moduleNamePattern.MatchString(request.ID) {
		fields["id"] = "invalid"
	}
	if _, err := semver.StrictNewVersion(request.Version); err != nil {
		fields["version"] = "invalid"
	}
	if _, err := semver.NewConstraint(request.Core); err != nil {
		fields["core"] = "invalid"
	}
	if match := apiPrefixPattern.FindStringSubmatch(request.APIPrefix); match == nil || match[1] != request.ID {
		fields["apiPrefix"] = "invalid"
	}
	if !areRequirementsValid(request.Requires) {
		fields["requires"] = "invalid"
	}
	if !arePermissionsValid(request.ID, request.Permissions) {
		fields["permissions"] = "invalid"
	}
	if len(fields) > 0 {
		return Registration{}, platform.NewValidationError(fields)
	}
	return request.toRegistration(), nil
}

func areRequirementsValid(requirements []RequirementRequest) bool {
	seen := map[string]bool{}
	for _, requirement := range requirements {
		if !moduleNamePattern.MatchString(requirement.ID) || seen[requirement.ID] {
			return false
		}
		if _, err := semver.NewConstraint(requirement.Version); err != nil {
			return false
		}
		seen[requirement.ID] = true
	}
	return true
}

func arePermissionsValid(moduleName string, permissions []PermissionRequest) bool {
	seen := map[string]bool{}
	for _, permission := range permissions {
		if !permissionNamePattern.MatchString(permission.Name) || !strings.HasPrefix(permission.Name, moduleName+".") || seen[permission.Name] {
			return false
		}
		seen[permission.Name] = true
	}
	return true
}

func (request RegisterModuleRequest) toRegistration() Registration {
	registration := Registration{
		ModuleName: request.ID,
		Version:    request.Version,
		CoreRange:  request.Core,
		APIPrefix:  request.APIPrefix,
		Requires:   make([]Requirement, 0, len(request.Requires)),
	}
	for _, requirement := range request.Requires {
		registration.Requires = append(registration.Requires, Requirement{ModuleName: requirement.ID, VersionRange: requirement.Version})
	}
	for _, permission := range request.Permissions {
		registration.Permissions = append(registration.Permissions, PermissionDeclaration{
			Name: permission.Name, DefaultRoles: permission.DefaultRoles,
		})
	}
	return registration
}
