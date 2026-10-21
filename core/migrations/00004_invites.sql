-- +goose Up
CREATE TABLE invites (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token              text NOT NULL UNIQUE DEFAULT public_token(),
    email              text NOT NULL,
    role_id            bigint NOT NULL REFERENCES roles (id),
    invited_by_user_id bigint NOT NULL REFERENCES users (id),
    secret_hash        bytea NOT NULL UNIQUE,
    created_at         timestamptz NOT NULL,
    expires_at         timestamptz NOT NULL,
    accepted_at        timestamptz,
    accepted_user_id   bigint REFERENCES users (id),
    revoked_at         timestamptz
);

CREATE INDEX invites_email_idx ON invites (email);

-- +goose Down
DROP TABLE invites;
