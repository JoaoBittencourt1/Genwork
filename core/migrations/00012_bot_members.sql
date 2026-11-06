-- +goose Up
ALTER TABLE users ADD COLUMN kind text NOT NULL DEFAULT 'human' CHECK (kind IN ('human', 'bot'));
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_human_credentials_check
    CHECK ((kind = 'human') = (password_hash IS NOT NULL AND email IS NOT NULL));

DROP INDEX users_email_active_key;
CREATE UNIQUE INDEX users_email_active_key ON users (email) WHERE deleted_at IS NULL AND kind = 'human';

CREATE TABLE bot_credentials (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token      text NOT NULL UNIQUE DEFAULT public_token(),
    user_id    bigint NOT NULL UNIQUE REFERENCES users (id),
    secret_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    rotated_at timestamptz
);

-- A bot member's token request starts a session of the bot client type that lives as long as its access token.
ALTER TABLE sessions DROP CONSTRAINT sessions_client_type_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_type_check CHECK (client_type IN ('web', 'native', 'oauth', 'bot'));

-- +goose Down
DELETE FROM refresh_tokens USING sessions WHERE refresh_tokens.session_id = sessions.id AND sessions.client_type = 'bot';
DELETE FROM sessions WHERE client_type = 'bot';
ALTER TABLE sessions DROP CONSTRAINT sessions_client_type_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_type_check CHECK (client_type IN ('web', 'native', 'oauth'));
DROP TABLE bot_credentials;
DROP INDEX users_email_active_key;
DELETE FROM users WHERE kind = 'bot';
CREATE UNIQUE INDEX users_email_active_key ON users (email) WHERE deleted_at IS NULL;
ALTER TABLE users DROP CONSTRAINT users_human_credentials_check;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP COLUMN kind;
