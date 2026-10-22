package modules

import (
	"net/http"
	"slices"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

var (
	ErrInvalidModuleSecret     = platform.NewAPIError(http.StatusUnauthorized, "invalid_module_secret")
	ErrIncompatibleCoreVersion = platform.NewAPIError(http.StatusConflict, "incompatible_core_version")
	ErrMissingRequirement      = platform.NewAPIError(http.StatusConflict, "missing_requirement")
)

type Module struct {
	ID        int64
	Token     string
	Name      string
	Version   string
	APIPrefix string
	Enabled   bool
}

type Requirement struct {
	ModuleName   string `json:"id"`
	VersionRange string `json:"version"`
}

type PermissionDeclaration struct {
	Name         string
	DefaultRoles []string
}

type Registration struct {
	ModuleName  string
	Version     string
	CoreRange   string
	APIPrefix   string
	Requires    []Requirement
	Permissions []PermissionDeclaration
	At          time.Time
}

func (registration Registration) PermissionNames() []string {
	names := make([]string, 0, len(registration.Permissions))
	for _, permission := range registration.Permissions {
		names = append(names, permission.Name)
	}
	return names
}

func (registration Registration) DefaultRoleNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, permission := range registration.Permissions {
		for _, role := range permission.DefaultRoles {
			if !seen[role] {
				seen[role] = true
				names = append(names, role)
			}
		}
	}
	return names
}

type PermissionGrant struct {
	PermissionName string
	RoleName       string
}

// NewlyDeclaredGrants lists the default roles each permission declares now but did not declare before, so a manifest
// that adds a role grants it once, while a grant an admin removed is never restored.
func (registration Registration) NewlyDeclaredGrants(previouslyDeclared map[string][]string) []PermissionGrant {
	var grants []PermissionGrant
	for _, permission := range registration.Permissions {
		for _, role := range permission.DefaultRoles {
			if !slices.Contains(previouslyDeclared[permission.Name], role) {
				grants = append(grants, PermissionGrant{PermissionName: permission.Name, RoleName: role})
			}
		}
	}
	return grants
}
