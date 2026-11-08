-- +goose Up
CREATE TABLE account_sign_ins (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token           text NOT NULL UNIQUE,
    account_id      bigint NOT NULL REFERENCES ai_accounts (id),
    state           text NOT NULL CHECK (state IN ('requested', 'awaiting_user', 'code_submitted', 'verifying', 'succeeded', 'failed', 'cancelled', 'expired')),
    sign_in_url     text,
    user_code       text,
    expects_code    boolean NOT NULL DEFAULT false,
    code_ciphertext bytea,
    failure         text,
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL
);

CREATE UNIQUE INDEX account_sign_ins_active_key ON account_sign_ins (account_id)
    WHERE state IN ('requested', 'awaiting_user', 'code_submitted', 'verifying');
CREATE INDEX account_sign_ins_account_idx ON account_sign_ins (account_id, created_at DESC);

CREATE TABLE mcp_server_oauth_connections (
    id                       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mcp_server_id            bigint NOT NULL UNIQUE REFERENCES mcp_servers (id),
    status                   text NOT NULL CHECK (status IN ('pending', 'connected', 'error')),
    issuer                   text,
    authorization_endpoint   text,
    token_endpoint           text,
    resource                 text,
    scope                    text,
    client_id                text,
    client_secret_ciphertext bytea,
    token_endpoint_auth_method text,
    access_token_ciphertext  bytea,
    refresh_token_ciphertext bytea,
    expires_at               timestamptz,
    connected_at             timestamptz,
    last_error               text,
    created_at               timestamptz NOT NULL,
    updated_at               timestamptz NOT NULL
);

CREATE TABLE mcp_server_oauth_authorizations (
    id                     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token                  text NOT NULL UNIQUE,
    mcp_server_id          bigint NOT NULL REFERENCES mcp_servers (id),
    state_hash             bytea NOT NULL UNIQUE,
    code_verifier_ciphertext bytea NOT NULL,
    expires_at             timestamptz NOT NULL,
    consumed_at            timestamptz,
    created_at             timestamptz NOT NULL
);

-- +goose Down
DROP TABLE mcp_server_oauth_authorizations;
DROP TABLE mcp_server_oauth_connections;
DROP TABLE account_sign_ins;
