-- +goose Up
CREATE TABLE oauth_clients (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token         text NOT NULL UNIQUE DEFAULT public_token(),
    name          text NOT NULL,
    redirect_uris text[] NOT NULL,
    created_at    timestamptz NOT NULL,
    deleted_at    timestamptz
);

CREATE TABLE oauth_authorization_codes (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token          text NOT NULL UNIQUE DEFAULT public_token(),
    code_hash      bytea NOT NULL UNIQUE,
    client_id      bigint NOT NULL REFERENCES oauth_clients (id),
    user_id        bigint NOT NULL REFERENCES users (id),
    redirect_uri   text NOT NULL,
    code_challenge text NOT NULL,
    scope          text NOT NULL,
    expires_at     timestamptz NOT NULL,
    used_at        timestamptz,
    session_id     bigint REFERENCES sessions (id),
    created_at     timestamptz NOT NULL
);

CREATE INDEX oauth_authorization_codes_client_id_idx ON oauth_authorization_codes (client_id);
CREATE INDEX oauth_authorization_codes_user_id_idx ON oauth_authorization_codes (user_id);

-- An assistant's grant is a session of the oauth client type, bound to the client that obtained it.
ALTER TABLE sessions ADD COLUMN oauth_client_id bigint REFERENCES oauth_clients (id);
ALTER TABLE sessions DROP CONSTRAINT sessions_client_type_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_type_check CHECK (client_type IN ('web', 'native', 'oauth'));
ALTER TABLE sessions ADD CONSTRAINT sessions_oauth_client_check CHECK ((client_type = 'oauth') = (oauth_client_id IS NOT NULL));

CREATE INDEX sessions_oauth_client_id_idx ON sessions (oauth_client_id);

-- +goose Down
DROP TABLE oauth_authorization_codes;
DELETE FROM refresh_tokens USING sessions WHERE refresh_tokens.session_id = sessions.id AND sessions.client_type = 'oauth';
DELETE FROM sessions WHERE client_type = 'oauth';
ALTER TABLE sessions DROP CONSTRAINT sessions_oauth_client_check;
ALTER TABLE sessions DROP CONSTRAINT sessions_client_type_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_type_check CHECK (client_type IN ('web', 'native'));
ALTER TABLE sessions DROP COLUMN oauth_client_id;
DROP TABLE oauth_clients;
