package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var ErrRoleNotFound = errors.New("role not found")

type RoleRepository struct{}

const roleSelect = `
SELECT roles.id, roles.token, roles.name,
       coalesce(array_agg(permissions.name ORDER BY permissions.name) FILTER (WHERE permissions.id IS NOT NULL), '{}')
FROM roles
LEFT JOIN role_permissions ON role_permissions.role_id = roles.id
LEFT JOIN permissions ON permissions.id = role_permissions.permission_id`

func (RoleRepository) FindByID(ctx context.Context, db platform.DBTX, id int64) (Role, error) {
	return scanSingleRole(db.QueryRow(ctx, roleSelect+` WHERE roles.id = $1 GROUP BY roles.id`, id))
}

func (RoleRepository) FindByToken(ctx context.Context, db platform.DBTX, token string) (Role, error) {
	return scanSingleRole(db.QueryRow(ctx, roleSelect+` WHERE roles.token = $1 GROUP BY roles.id`, token))
}

func (RoleRepository) FindByName(ctx context.Context, db platform.DBTX, name string) (Role, error) {
	return scanSingleRole(db.QueryRow(ctx, roleSelect+` WHERE roles.name = $1 GROUP BY roles.id`, name))
}

func (RoleRepository) List(ctx context.Context, db platform.DBTX) ([]Role, error) {
	rows, err := db.Query(ctx, roleSelect+` GROUP BY roles.id ORDER BY roles.name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	roles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Role, error) {
		return scanRole(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return roles, nil
}

func scanSingleRole(row pgx.Row) (Role, error) {
	role, err := scanRole(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrRoleNotFound
	}
	if err != nil {
		return Role{}, fmt.Errorf("find role: %w", err)
	}
	return role, nil
}

func scanRole(row pgx.Row) (Role, error) {
	var role Role
	err := row.Scan(&role.ID, &role.Token, &role.Name, &role.Permissions)
	return role, err
}
