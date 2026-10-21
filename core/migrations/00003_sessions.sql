-- +goose Up
CREATE TABLE sessions (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token               text NOT NULL UNIQUE DEFAULT public_token(),
    user_id             bigint NOT NULL REFERENCES users (id),
    client_type         text NOT NULL CHECK (client_type IN ('web', 'native')),
    device_name         text,
    created_at          timestamptz NOT NULL,
    last_used_at        timestamptz NOT NULL,
    idle_expires_at     timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at          timestamptz
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE refresh_tokens (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token      text NOT NULL UNIQUE DEFAULT public_token(),
    session_id bigint NOT NULL REFERENCES sessions (id),
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE INDEX refresh_tokens_session_id_idx ON refresh_tokens (session_id);

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE sessions;
