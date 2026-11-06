-- +goose Up
CREATE TABLE bots (
    id                       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token                    text NOT NULL UNIQUE,
    name                     text NOT NULL,
    kind                     text NOT NULL CHECK (kind IN ('dev', 'reviewer', 'assistant')),
    account_id               bigint NOT NULL REFERENCES ai_accounts (id),
    model                    text NOT NULL,
    effort                   text NOT NULL CHECK (effort IN ('default', 'low', 'medium', 'high')),
    max_turns                integer NOT NULL CHECK (max_turns BETWEEN 1 AND 500),
    timeout_minutes          integer NOT NULL CHECK (timeout_minutes BETWEEN 5 AND 720),
    instructions             text,
    member_token             text NOT NULL,
    member_secret_ciphertext bytea,
    disabled_at              timestamptz,
    created_at               timestamptz NOT NULL,
    updated_at               timestamptz NOT NULL,
    deleted_at               timestamptz
);

CREATE UNIQUE INDEX bots_name_active_key ON bots (name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX bots_member_token_active_key ON bots (member_token) WHERE deleted_at IS NULL;
CREATE INDEX bots_account_id_idx ON bots (account_id);

CREATE TABLE bot_skills (
    bot_id   bigint NOT NULL REFERENCES bots (id),
    skill_id bigint NOT NULL REFERENCES skills (id),
    PRIMARY KEY (bot_id, skill_id)
);

CREATE TABLE bot_mcp_servers (
    bot_id        bigint NOT NULL REFERENCES bots (id),
    mcp_server_id bigint NOT NULL REFERENCES mcp_servers (id),
    PRIMARY KEY (bot_id, mcp_server_id)
);

-- +goose Down
DROP TABLE bot_mcp_servers;
DROP TABLE bot_skills;
DROP TABLE bots;
