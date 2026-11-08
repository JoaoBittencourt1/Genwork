package githubsync

import (
	"context"
	"fmt"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/jackc/pgx/v5"
)

// RepositoryScopeRepository answers which repositories gentasks works with: those set on a board for bots, and those of
// pull requests linked to a task. Only those may receive a token.
type RepositoryScopeRepository struct{}

// ListKnown returns, lowercased, which of the repositories gentasks knows.
func (RepositoryScopeRepository) ListKnown(ctx context.Context, db platform.DBTX, repositories []string) (map[string]bool, error) {
	lowered := make([]string, 0, len(repositories))
	for _, repository := range repositories {
		lowered = append(lowered, strings.ToLower(repository))
	}
	rows, err := db.Query(ctx, `
		SELECT lower(settings.repository) FROM board_github_settings settings
		JOIN boards ON boards.id = settings.board_id AND boards.deleted_at IS NULL
		WHERE lower(settings.repository) = ANY($1)
		UNION
		SELECT lower(github_pull_requests.repository) FROM github_pull_requests
		JOIN task_pull_requests ON task_pull_requests.pull_request_id = github_pull_requests.id AND task_pull_requests.deleted_at IS NULL
		JOIN tasks ON tasks.id = task_pull_requests.task_id AND tasks.deleted_at IS NULL
		WHERE lower(github_pull_requests.repository) = ANY($1)`, lowered)
	if err != nil {
		return nil, fmt.Errorf("list known repositories: %w", err)
	}
	known, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("list known repositories: %w", err)
	}
	result := make(map[string]bool, len(known))
	for _, repository := range known {
		result[repository] = true
	}
	return result, nil
}
