-- +goose Up
ALTER TABLE boards ADD COLUMN task_prefix text;
ALTER TABLE boards ADD COLUMN next_task_number integer NOT NULL DEFAULT 1 CHECK (next_task_number > 0);
ALTER TABLE tasks ADD COLUMN number integer CHECK (number > 0);

-- Existing boards get the prefix boards.DeriveTaskPrefix would pick; boards sharing one get a numeric suffix in creation order.
WITH derived AS (
    SELECT id,
           coalesce(nullif(upper(left(regexp_replace(regexp_replace(name, '[^A-Za-z0-9]', '', 'g'), '^[0-9]+', ''), 3)), ''), 'TASK') AS base
    FROM boards
), ranked AS (
    SELECT id, base, row_number() OVER (PARTITION BY base ORDER BY id) AS rank FROM derived
)
UPDATE boards
SET task_prefix = CASE WHEN ranked.rank = 1 THEN ranked.base ELSE ranked.base || ranked.rank END
FROM ranked
WHERE boards.id = ranked.id;

-- Existing tasks, deleted ones included, are numbered per board in creation order.
UPDATE tasks
SET number = numbered.number
FROM (SELECT id, row_number() OVER (PARTITION BY board_id ORDER BY created_at, id) AS number FROM tasks) AS numbered
WHERE tasks.id = numbered.id;

UPDATE boards
SET next_task_number = coalesce((SELECT max(number) FROM tasks WHERE tasks.board_id = boards.id), 0) + 1;

ALTER TABLE boards ALTER COLUMN task_prefix SET NOT NULL;
ALTER TABLE boards ADD CONSTRAINT boards_task_prefix_format CHECK (task_prefix ~ '^[A-Z][A-Z0-9]{0,9}$');
ALTER TABLE tasks ALTER COLUMN number SET NOT NULL;

CREATE UNIQUE INDEX boards_task_prefix_idx ON boards (task_prefix) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tasks_board_number_idx ON tasks (board_id, number);

-- +goose Down
DROP INDEX tasks_board_number_idx;
DROP INDEX boards_task_prefix_idx;
ALTER TABLE tasks DROP COLUMN number;
ALTER TABLE boards DROP COLUMN next_task_number;
ALTER TABLE boards DROP COLUMN task_prefix;
