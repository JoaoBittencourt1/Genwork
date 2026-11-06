-- +goose Up
CREATE TABLE conversations (
    id                         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token                      text NOT NULL UNIQUE,
    kind                       text NOT NULL CHECK (kind IN ('task', 'admin_chat')),
    task_token                 text,
    owner_user_token           text,
    title                      text,
    bot_id                     bigint REFERENCES bots (id),
    delegated_session          text,
    delegated_refresh_ciphertext bytea,
    created_at                 timestamptz NOT NULL,
    updated_at                 timestamptz NOT NULL,
    deleted_at                 timestamptz,
    CHECK ((kind = 'task') = (task_token IS NOT NULL))
);

CREATE UNIQUE INDEX conversations_task_active_key ON conversations (task_token) WHERE deleted_at IS NULL AND kind = 'task';
CREATE INDEX conversations_owner_idx ON conversations (owner_user_token) WHERE deleted_at IS NULL;

CREATE TABLE runs (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token              text NOT NULL UNIQUE,
    conversation_id    bigint NOT NULL REFERENCES conversations (id),
    bot_id             bigint NOT NULL REFERENCES bots (id),
    kind               text NOT NULL CHECK (kind IN ('task', 'review', 'chat')),
    state              text NOT NULL CHECK (state IN ('queued', 'running', 'waiting_for_input', 'succeeded', 'failed', 'cancelled')),
    failure_reason     text,
    requester_user_token text NOT NULL,
    runner_id          bigint REFERENCES runners (id),
    cli_session_id     text,
    spec               jsonb NOT NULL DEFAULT '{}',
    waiting_since      timestamptz,
    timeout_at         timestamptz,
    created_at         timestamptz NOT NULL,
    claimed_at         timestamptz,
    ended_at           timestamptz,
    CHECK ((state = 'failed') = (failure_reason IS NOT NULL))
);

-- One active run per conversation and kind: a task has one active task run and one active review.
CREATE UNIQUE INDEX runs_active_per_conversation_kind_key ON runs (conversation_id, kind)
    WHERE state IN ('queued', 'running', 'waiting_for_input') AND kind <> 'chat';
CREATE INDEX runs_queue_idx ON runs (created_at) WHERE state = 'queued';
CREATE INDEX runs_runner_idx ON runs (runner_id) WHERE state IN ('running', 'waiting_for_input');
CREATE INDEX runs_bot_idx ON runs (bot_id);

CREATE TABLE conversation_entries (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token           text NOT NULL UNIQUE,
    conversation_id bigint NOT NULL REFERENCES conversations (id),
    run_id          bigint REFERENCES runs (id),
    sequence        bigint NOT NULL,
    run_sequence    bigint,
    type            text NOT NULL CHECK (type IN ('human_message', 'status', 'assistant_message', 'tool_use', 'result', 'error')),
    author_token    text,
    payload         jsonb NOT NULL,
    delivered_at    timestamptz,
    created_at      timestamptz NOT NULL,
    UNIQUE (conversation_id, sequence)
);

-- Runner events are idempotent by their per-run sequence.
CREATE UNIQUE INDEX conversation_entries_run_sequence_key ON conversation_entries (run_id, run_sequence) WHERE run_sequence IS NOT NULL;

-- +goose Down
DROP TABLE conversation_entries;
DROP TABLE runs;
DROP TABLE conversations;
