-- +goose Up
CREATE TABLE password_resets (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token       text NOT NULL UNIQUE DEFAULT public_token(),
    user_id     bigint NOT NULL REFERENCES users (id),
    secret_hash bytea NOT NULL UNIQUE,
    created_at  timestamptz NOT NULL,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);

CREATE INDEX password_resets_user_idx ON password_resets (user_id);

-- +goose Down
DROP TABLE password_resets;
