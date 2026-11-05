-- +goose Up
ALTER TABLE tasks ADD COLUMN assignee_kind text NOT NULL DEFAULT 'human' CHECK (assignee_kind IN ('human', 'bot'));
ALTER TABLE board_github_settings ADD COLUMN repository text CHECK (repository ~ '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$');

-- +goose Down
ALTER TABLE board_github_settings DROP COLUMN repository;
ALTER TABLE tasks DROP COLUMN assignee_kind;
