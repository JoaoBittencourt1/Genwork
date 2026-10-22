package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

type ModuleRepository struct{}

const moduleColumns = `id, token, name, version, api_prefix, enabled`

func (ModuleRepository) LockRegistry(ctx context.Context, db platform.DBTX) error {
	if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('genwork.module_registry'))`); err != nil {
		return fmt.Errorf("lock module registry: %w", err)
	}
	return nil
}

func (ModuleRepository) Upsert(ctx context.Context, db platform.DBTX, registration Registration) (Module, error) {
	requirements, err := json.Marshal(registration.Requires)
	if err != nil {
		return Module{}, fmt.Errorf("encode requirements: %w", err)
	}
	row := db.QueryRow(ctx, `
		INSERT INTO modules (name, version, core_range, api_prefix, requirements, registered_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
			version = EXCLUDED.version,
			core_range = EXCLUDED.core_range,
			api_prefix = EXCLUDED.api_prefix,
			requirements = EXCLUDED.requirements,
			registered_at = EXCLUDED.registered_at
		RETURNING `+moduleColumns,
		registration.ModuleName, registration.Version, registration.CoreRange, registration.APIPrefix, requirements, registration.At)
	module, err := scanModule(row)
	if err != nil {
		return Module{}, fmt.Errorf("upsert module: %w", err)
	}
	return module, nil
}

func (ModuleRepository) FindByNames(ctx context.Context, db platform.DBTX, names []string) ([]Module, error) {
	return listModules(ctx, db, `SELECT `+moduleColumns+` FROM modules WHERE name = ANY($1) ORDER BY name`, names)
}

func (ModuleRepository) ListEnabled(ctx context.Context, db platform.DBTX) ([]Module, error) {
	return listModules(ctx, db, `SELECT `+moduleColumns+` FROM modules WHERE enabled ORDER BY name`)
}

// SetEnabled reports false when no module with that name is registered.
func (ModuleRepository) SetEnabled(ctx context.Context, db platform.DBTX, name string, enabled bool) (bool, error) {
	tag, err := db.Exec(ctx, `UPDATE modules SET enabled = $2 WHERE name = $1`, name, enabled)
	if err != nil {
		return false, fmt.Errorf("set module state: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// InsertMissingPermissions adds the permissions core has not seen, with no declared default role yet.
func (ModuleRepository) InsertMissingPermissions(ctx context.Context, db platform.DBTX, moduleID int64, names []string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO permissions (name, module_id, default_roles)
		SELECT unnest($2::text[]), $1, '{}'
		ON CONFLICT (name) DO NOTHING`, moduleID, names)
	if err != nil {
		return fmt.Errorf("insert module permissions: %w", err)
	}
	return nil
}

// FindDeclaredDefaultRoles maps each of the module's permissions to the default roles its manifest last declared.
func (ModuleRepository) FindDeclaredDefaultRoles(ctx context.Context, db platform.DBTX, moduleID int64) (map[string][]string, error) {
	rows, err := db.Query(ctx, `SELECT name, COALESCE(default_roles, '{}') FROM permissions WHERE module_id = $1`, moduleID)
	if err != nil {
		return nil, fmt.Errorf("find declared default roles: %w", err)
	}
	declared := map[string][]string{}
	var name string
	var roles []string
	_, err = pgx.ForEachRow(rows, []any{&name, &roles}, func() error {
		declared[name] = roles
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find declared default roles: %w", err)
	}
	return declared, nil
}

func (ModuleRepository) RecordDeclaredDefaultRoles(ctx context.Context, db platform.DBTX, permissions []PermissionDeclaration) error {
	names := make([]string, 0, len(permissions))
	roles := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		encoded, err := json.Marshal(permission.DefaultRoles)
		if err != nil {
			return fmt.Errorf("record declared default roles: %w", err)
		}
		names = append(names, permission.Name)
		roles = append(roles, string(encoded))
	}
	_, err := db.Exec(ctx, `
		UPDATE permissions
		SET default_roles = ARRAY(SELECT jsonb_array_elements_text(declared.roles::jsonb))
		FROM unnest($1::text[], $2::text[]) AS declared (name, roles)
		WHERE permissions.name = declared.name`, names, roles)
	if err != nil {
		return fmt.Errorf("record declared default roles: %w", err)
	}
	return nil
}

func (ModuleRepository) GrantPermissions(ctx context.Context, db platform.DBTX, grants []PermissionGrant) error {
	if len(grants) == 0 {
		return nil
	}
	permissionNames := make([]string, 0, len(grants))
	roleNames := make([]string, 0, len(grants))
	for _, grant := range grants {
		permissionNames = append(permissionNames, grant.PermissionName)
		roleNames = append(roleNames, grant.RoleName)
	}
	_, err := db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT roles.id, permissions.id
		FROM unnest($1::text[], $2::text[]) AS requested (permission_name, role_name)
		JOIN permissions ON permissions.name = requested.permission_name
		JOIN roles ON roles.name = requested.role_name
		ON CONFLICT (role_id, permission_id) DO NOTHING`, permissionNames, roleNames)
	if err != nil {
		return fmt.Errorf("grant module permissions: %w", err)
	}
	return nil
}

// SyncRetiredPermissions retires the module's permissions missing from its manifest and revives the ones that came back.
func (ModuleRepository) SyncRetiredPermissions(ctx context.Context, db platform.DBTX, moduleID int64, declaredNames []string, at time.Time) error {
	_, err := db.Exec(ctx, `
		UPDATE permissions
		SET retired_at = CASE WHEN name = ANY($2) THEN NULL ELSE coalesce(retired_at, $3) END
		WHERE module_id = $1`, moduleID, declaredNames, at)
	if err != nil {
		return fmt.Errorf("sync retired permissions: %w", err)
	}
	return nil
}

func listModules(ctx context.Context, db platform.DBTX, query string, arguments ...any) ([]Module, error) {
	rows, err := db.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	modules, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Module, error) {
		return scanModule(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	return modules, nil
}

func scanModule(row pgx.Row) (Module, error) {
	var module Module
	err := row.Scan(&module.ID, &module.Token, &module.Name, &module.Version, &module.APIPrefix, &module.Enabled)
	return module, err
}
