-- +goose Up
-- At most one GitHub App per installation. Disconnecting deletes the row physically, a documented exception to rule 22:
-- a soft-deleted row would keep the App's private key in the database.
CREATE TABLE github_apps (
    id                        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token                     text NOT NULL UNIQUE,
    github_app_id             bigint NOT NULL,
    slug                      text NOT NULL,
    owner_login               text NOT NULL,
    app_url                   text NOT NULL,
    webhook_secret_ciphertext bytea NOT NULL,
    private_key_ciphertext    bytea NOT NULL,
    connected_by              text NOT NULL,
    created_at                timestamptz NOT NULL,
    updated_at                timestamptz NOT NULL
);

CREATE UNIQUE INDEX github_apps_single_row_idx ON github_apps ((true));

-- A board without a row uses the defaults; the row lives and dies with its board, so it has no deleted_at.
CREATE TABLE board_github_settings (
    id                         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token                      text NOT NULL UNIQUE,
    board_id                   bigint NOT NULL UNIQUE REFERENCES boards (id),
    opened_column_id           bigint REFERENCES board_columns (id),
    review_requested_column_id bigint REFERENCES board_columns (id),
    merged_column_id           bigint REFERENCES board_columns (id),
    final_branches             text[] NOT NULL CHECK (cardinality(final_branches) BETWEEN 1 AND 20),
    created_at                 timestamptz NOT NULL,
    updated_at                 timestamptz NOT NULL
);

CREATE TABLE task_pull_requests (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token               text NOT NULL UNIQUE,
    task_id             bigint NOT NULL REFERENCES tasks (id),
    repository          text NOT NULL,
    pull_request_number integer NOT NULL CHECK (pull_request_number > 0),
    last_moved_event    text NOT NULL CHECK (last_moved_event IN ('opened', 'review_requested', 'merged')),
    moved_at            timestamptz NOT NULL,
    merge_moved_at      timestamptz,
    created_at          timestamptz NOT NULL,
    updated_at          timestamptz NOT NULL,
    UNIQUE (task_id, repository, pull_request_number)
);

-- +goose Down
DROP TABLE task_pull_requests;
DROP TABLE board_github_settings;
DROP TABLE github_apps;
