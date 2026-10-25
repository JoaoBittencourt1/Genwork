package platform_test

import (
	"context"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const githubSyncMigrationVersion = 4

type migratedLink struct {
	TaskID         int64
	Repository     string
	Number         int
	Token          string
	LastMovedEvent *string
	MergeMovedAt   *string
	Title          *string
	State          *string
}

func TestTaskPullRequestSnapshotsMigrationKeepsExistingLinks(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	ctx := context.Background()
	pool := openSchemaAtMigration(t, databaseURL, schema, githubSyncMigrationVersion)
	seedLinksBeforeSnapshots(t, pool)

	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var snapshots int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM github_pull_requests`).Scan(&snapshots); err != nil || snapshots != 2 {
		t.Fatalf("snapshots = %d, err = %v", snapshots, err)
	}
	links := readMigratedLinks(t, pool)
	want := []migratedLink{
		{TaskID: 1, Repository: "acme/api", Number: 12, Token: "link-a"},
		{TaskID: 1, Repository: "acme/web", Number: 3, Token: "link-c"},
		{TaskID: 2, Repository: "acme/api", Number: 12, Token: "link-a"},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %+v", links)
	}
	for index, link := range links {
		expected := want[index]
		if link.TaskID != expected.TaskID || link.Repository != expected.Repository || link.Number != expected.Number ||
			link.Token != expected.Token || link.Title != nil || link.State != nil || link.LastMovedEvent == nil {
			t.Errorf("link %d = %+v, want %+v", index, link, expected)
		}
	}
	if links[1].MergeMovedAt == nil || *links[1].LastMovedEvent != "merged" {
		t.Errorf("merge link lost its lock: %+v", links[1])
	}
}

func TestTaskPullRequestSnapshotsMigrationRollsBack(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	ctx := context.Background()
	pool := openSchemaAtMigration(t, databaseURL, schema, githubSyncMigrationVersion)
	seedLinksBeforeSnapshots(t, pool)
	if err := platform.Migrate(ctx, databaseURL, schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO task_pull_requests (token, task_id, pull_request_id, created_at, updated_at)
		SELECT 'link-without-move', 2, id, now(), now() FROM github_pull_requests WHERE repository = 'acme/web'`)
	if err != nil {
		t.Fatalf("link without move: %v", err)
	}

	if _, err := newMigrationProvider(t, pool).DownTo(ctx, githubSyncMigrationVersion); err != nil {
		t.Fatalf("roll back: %v", err)
	}

	rows, _ := pool.Query(ctx, `SELECT token FROM task_pull_requests ORDER BY token`)
	tokens, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(tokens) != 3 || tokens[0] != "link-a" || tokens[1] != "link-b" || tokens[2] != "link-c" {
		t.Errorf("tokens after rollback = %v, err = %v", tokens, err)
	}
	var repository string
	if err := pool.QueryRow(ctx, `SELECT repository FROM task_pull_requests WHERE token = 'link-c'`).Scan(&repository); err != nil || repository != "acme/web" {
		t.Errorf("repository = %q, err = %v", repository, err)
	}
}

func newMigrationProvider(t *testing.T, pool *pgxpool.Pool) *goose.Provider {
	t.Helper()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	return provider
}

// seedLinksBeforeSnapshots: two tasks moved by acme/api#12, and the first one also locked by the merge of acme/web#3.
func seedLinksBeforeSnapshots(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO boards (token, name, task_prefix, next_task_number, created_by, created_at, updated_at) VALUES
			('b1', 'Platform', 'GEN', 3, 'u', now(), now());
		INSERT INTO board_columns (token, board_id, preset, position, created_at, updated_at) VALUES
			('c1', 1, 'backlog', 0, now(), now());
		INSERT INTO tasks (token, board_id, column_id, number, title, position, created_by, created_at, updated_at) VALUES
			('t1', 1, 1, 1, 'Sign in', 0, 'u', now(), now()),
			('t2', 1, 1, 2, 'Profile', 1, 'u', now(), now());
		INSERT INTO task_pull_requests (token, task_id, repository, pull_request_number, last_moved_event, moved_at, merge_moved_at, created_at, updated_at) VALUES
			('link-b', 2, 'acme/api', 12, 'opened', now(), NULL, now(), now()),
			('link-a', 1, 'acme/api', 12, 'review_requested', now(), NULL, now(), now()),
			('link-c', 1, 'acme/web', 3, 'merged', now(), now(), now(), now());`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func readMigratedLinks(t *testing.T, pool *pgxpool.Pool) []migratedLink {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT links.task_id, pull_requests.repository, pull_requests.number, pull_requests.token, links.last_moved_event,
		       links.merge_moved_at::text, pull_requests.title, pull_requests.state
		FROM task_pull_requests AS links
		JOIN github_pull_requests AS pull_requests ON pull_requests.id = links.pull_request_id
		ORDER BY links.task_id, pull_requests.repository`)
	if err != nil {
		t.Fatalf("read links: %v", err)
	}
	links, err := pgx.CollectRows(rows, pgx.RowToStructByPos[migratedLink])
	if err != nil {
		t.Fatalf("read links: %v", err)
	}
	return links
}
