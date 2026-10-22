package access

import (
	"context"
	"fmt"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type PermissionRepository struct{}

// FindEffectiveForUser returns the user's role permissions minus those of the modules restricted for the user.
// Core permissions have no module, so restrictions never remove them.
func (PermissionRepository) FindEffectiveForUser(ctx context.Context, db platform.DBTX, userID int64) ([]string, error) {
	var permissions []string
	err := db.QueryRow(ctx, `
		SELECT coalesce(array_agg(permissions.name ORDER BY permissions.name), '{}')
		FROM users
		JOIN role_permissions ON role_permissions.role_id = users.role_id
		JOIN permissions ON permissions.id = role_permissions.permission_id AND permissions.retired_at IS NULL
		WHERE users.id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM user_module_restrictions
		      WHERE user_module_restrictions.user_id = users.id
		        AND user_module_restrictions.module_id = permissions.module_id
		        AND user_module_restrictions.deleted_at IS NULL)`, userID).Scan(&permissions)
	if err != nil {
		return nil, fmt.Errorf("find user permissions: %w", err)
	}
	return permissions, nil
}
