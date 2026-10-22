package access

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5"
)

var ErrUnknownModule = errors.New("unknown module")

// ModuleRestrictionRepository stores the modules a user may not access; no row means access (blocklist).
type ModuleRestrictionRepository struct{}

// Replace makes moduleNames the user's whole set of restricted modules, soft-deleting the restrictions it lifts.
func (ModuleRestrictionRepository) Replace(ctx context.Context, db platform.DBTX, userID int64, moduleNames []string, at time.Time) error {
	distinctNames := slices.Compact(slices.Sorted(slices.Values(moduleNames)))
	var knownCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM modules WHERE name = ANY($1)`, distinctNames).Scan(&knownCount); err != nil {
		return fmt.Errorf("check restricted modules: %w", err)
	}
	if knownCount != len(distinctNames) {
		return ErrUnknownModule
	}
	_, err := db.Exec(ctx, `
		UPDATE user_module_restrictions SET deleted_at = $3
		WHERE user_id = $1 AND deleted_at IS NULL
		  AND module_id NOT IN (SELECT id FROM modules WHERE name = ANY($2))`, userID, distinctNames, at)
	if err != nil {
		return fmt.Errorf("lift module restrictions: %w", err)
	}
	_, err = db.Exec(ctx, `
		INSERT INTO user_module_restrictions (user_id, module_id, created_at)
		SELECT $1, modules.id, $3 FROM modules WHERE modules.name = ANY($2)
		ON CONFLICT (user_id, module_id) WHERE deleted_at IS NULL DO NOTHING`, userID, distinctNames, at)
	if err != nil {
		return fmt.Errorf("add module restrictions: %w", err)
	}
	return nil
}

func (ModuleRestrictionRepository) ListModuleNamesByUserIDs(ctx context.Context, db platform.DBTX, userIDs []int64) (map[int64][]string, error) {
	rows, err := db.Query(ctx, `
		SELECT user_module_restrictions.user_id, modules.name
		FROM user_module_restrictions
		JOIN modules ON modules.id = user_module_restrictions.module_id
		WHERE user_module_restrictions.user_id = ANY($1) AND user_module_restrictions.deleted_at IS NULL
		ORDER BY modules.name`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("list module restrictions: %w", err)
	}
	namesByUser := map[int64][]string{}
	var userID int64
	var moduleName string
	_, err = pgx.ForEachRow(rows, []any{&userID, &moduleName}, func() error {
		namesByUser[userID] = append(namesByUser[userID], moduleName)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list module restrictions: %w", err)
	}
	return namesByUser, nil
}
