-- +goose Up
CREATE TABLE users (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token              text NOT NULL UNIQUE DEFAULT public_token(),
    email              text NOT NULL,
    name               text NOT NULL,
    password_hash      text NOT NULL,
    role_id            bigint NOT NULL REFERENCES roles (id),
    failed_login_count integer NOT NULL DEFAULT 0,
    locked_until       timestamptz,
    last_login_at      timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz
);

CREATE UNIQUE INDEX users_email_active_key ON users (email) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE users;
