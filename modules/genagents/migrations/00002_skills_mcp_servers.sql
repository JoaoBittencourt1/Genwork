-- +goose Up
CREATE TABLE skills (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token       text NOT NULL UNIQUE,
    name        text NOT NULL,
    description text NOT NULL,
    body        text NOT NULL,
    version     integer NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL,
    deleted_at  timestamptz
);

CREATE UNIQUE INDEX skills_name_active_key ON skills (name) WHERE deleted_at IS NULL;

CREATE TABLE skill_files (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token      text NOT NULL UNIQUE,
    skill_id   bigint NOT NULL REFERENCES skills (id),
    path       text NOT NULL,
    content    text NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (skill_id, path)
);

CREATE TABLE mcp_servers (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token              text NOT NULL UNIQUE,
    name               text NOT NULL,
    transport          text NOT NULL CHECK (transport IN ('http', 'stdio')),
    config             jsonb NOT NULL,
    secrets_ciphertext bytea,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    deleted_at         timestamptz
);

CREATE UNIQUE INDEX mcp_servers_name_active_key ON mcp_servers (name) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE mcp_servers;
DROP TABLE skill_files;
DROP TABLE skills;
