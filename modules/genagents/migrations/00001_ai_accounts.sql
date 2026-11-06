-- +goose Up
CREATE TABLE runners (
    id                      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token                   text NOT NULL UNIQUE,
    name                    text NOT NULL,
    secret_hash             bytea,
    registration_token_hash bytea,
    registration_expires_at timestamptz,
    last_heartbeat_at       timestamptz,
    container_engine        text,
    sandbox_image           text,
    custom_image            boolean NOT NULL DEFAULT false,
    directory_mode          boolean NOT NULL DEFAULT false,
    reported_tools          jsonb NOT NULL DEFAULT '{}',
    has_github_credentials  boolean NOT NULL DEFAULT false,
    max_concurrency         integer NOT NULL DEFAULT 2 CHECK (max_concurrency BETWEEN 1 AND 64),
    revoked_at              timestamptz,
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    deleted_at              timestamptz
);

CREATE UNIQUE INDEX runners_name_active_key ON runners (name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX runners_secret_hash_key ON runners (secret_hash) WHERE secret_hash IS NOT NULL;
CREATE UNIQUE INDEX runners_registration_token_hash_key ON runners (registration_token_hash) WHERE registration_token_hash IS NOT NULL;

CREATE TABLE ai_accounts (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token              text NOT NULL UNIQUE,
    name               text NOT NULL,
    provider           text NOT NULL CHECK (provider IN ('anthropic', 'openai', 'cursor', 'xai')),
    mode               text NOT NULL CHECK (mode IN ('subscription', 'api_key')),
    runner_id          bigint REFERENCES runners (id),
    api_key_ciphertext bytea,
    api_key_last4      text,
    status             text NOT NULL CHECK (status IN ('pending_sign_in', 'ready', 'error')),
    last_error         text,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    deleted_at         timestamptz,
    CHECK ((mode = 'subscription') = (runner_id IS NOT NULL)),
    CHECK (mode = 'subscription' OR deleted_at IS NOT NULL OR api_key_ciphertext IS NOT NULL)
);

CREATE UNIQUE INDEX ai_accounts_name_active_key ON ai_accounts (name) WHERE deleted_at IS NULL;

CREATE TABLE ai_account_models (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token        text NOT NULL UNIQUE,
    account_id   bigint NOT NULL REFERENCES ai_accounts (id),
    model_id     text NOT NULL,
    display_name text NOT NULL,
    created_at   timestamptz NOT NULL,
    deleted_at   timestamptz
);

CREATE UNIQUE INDEX ai_account_models_account_model_key ON ai_account_models (account_id, model_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE ai_account_models;
DROP TABLE ai_accounts;
DROP TABLE runners;
