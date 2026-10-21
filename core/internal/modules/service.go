package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/Masterminds/semver/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModuleService struct {
	db             *pgxpool.Pool
	modules        ModuleRepository
	roles          access.RoleRepository
	coreAPIVersion *semver.Version
	now            func() time.Time
}

func NewModuleService(db *pgxpool.Pool, coreAPIVersion string, now func() time.Time) *ModuleService {
	return &ModuleService{db: db, coreAPIVersion: semver.MustParse(coreAPIVersion), now: now}
}

func (service *ModuleService) Register(ctx context.Context, registration Registration) (Module, error) {
	if !satisfies(registration.CoreRange, service.coreAPIVersion) {
		return Module{}, ErrIncompatibleCoreVersion
	}
	registration.At = service.now()
	var module Module
	err := pgx.BeginFunc(ctx, service.db, func(tx pgx.Tx) error {
		if err := service.modules.LockRegistry(ctx, tx); err != nil {
			return err
		}
		if err := service.checkDefaultRoles(ctx, tx, registration); err != nil {
			return err
		}
		if err := service.checkRequirements(ctx, tx, registration.Requires); err != nil {
			return err
		}
		var err error
		module, err = service.modules.Upsert(ctx, tx, registration)
		if err != nil {
			return err
		}
		return service.syncPermissions(ctx, tx, module, registration)
	})
	return module, err
}

// SetEnabled is how the installer switches a module on or off in capabilities; registration never changes it.
func (service *ModuleService) SetEnabled(ctx context.Context, name string, enabled bool) (bool, error) {
	return service.modules.SetEnabled(ctx, service.db, name, enabled)
}

func (service *ModuleService) ListEnabled(ctx context.Context) ([]Module, error) {
	return service.modules.ListEnabled(ctx, service.db)
}

func (service *ModuleService) checkDefaultRoles(ctx context.Context, tx pgx.Tx, registration Registration) error {
	wanted := registration.DefaultRoleNames()
	existing, err := service.roles.FindExistingNames(ctx, tx, wanted)
	if err != nil {
		return err
	}
	if len(existing) != len(wanted) {
		return platform.NewValidationError(map[string]string{"permissions": "unknown_role"})
	}
	return nil
}

func (service *ModuleService) checkRequirements(ctx context.Context, tx pgx.Tx, requirements []Requirement) error {
	if len(requirements) == 0 {
		return nil
	}
	names := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		names = append(names, requirement.ModuleName)
	}
	registered, err := service.modules.FindByNames(ctx, tx, names)
	if err != nil {
		return err
	}
	byName := make(map[string]Module, len(registered))
	for _, module := range registered {
		byName[module.Name] = module
	}
	for _, requirement := range requirements {
		required, found := byName[requirement.ModuleName]
		if !found || !required.Enabled || !satisfies(requirement.VersionRange, semver.MustParse(required.Version)) {
			return ErrMissingRequirement
		}
	}
	return nil
}

func (service *ModuleService) syncPermissions(ctx context.Context, tx pgx.Tx, module Module, registration Registration) error {
	declared := registration.PermissionNames()
	inserted, err := service.modules.InsertMissingPermissions(ctx, tx, module.ID, declared)
	if err != nil {
		return err
	}
	if err := service.modules.GrantPermissions(ctx, tx, registration.DefaultGrantsFor(inserted)); err != nil {
		return err
	}
	return service.modules.SyncRetiredPermissions(ctx, tx, module.ID, declared, registration.At)
}

func satisfies(versionRange string, version *semver.Version) bool {
	constraint, err := semver.NewConstraint(versionRange)
	if err != nil {
		panic(fmt.Sprintf("version range %q was not validated: %v", versionRange, err))
	}
	return constraint.Check(version)
}
