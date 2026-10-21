-- +goose Up
ALTER TABLE tasks ADD COLUMN assigned_at timestamptz;

-- No assignment history exists, so tasks assigned before this migration take their creation time.
UPDATE tasks SET assigned_at = created_at WHERE assignee_user_token IS NOT NULL;

ALTER TABLE tasks ADD CONSTRAINT tasks_assigned_at_matches_assignee CHECK ((assigned_at IS NULL) = (assignee_user_token IS NULL));

CREATE INDEX tasks_assignee_assigned_idx ON tasks (assignee_user_token, assigned_at) WHERE position IS NOT NULL;

-- +goose Down
DROP INDEX tasks_assignee_assigned_idx;
ALTER TABLE tasks DROP CONSTRAINT tasks_assigned_at_matches_assignee;
ALTER TABLE tasks DROP COLUMN assigned_at;
