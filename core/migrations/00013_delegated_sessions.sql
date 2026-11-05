-- +goose Up
ALTER TABLE sessions ADD COLUMN delegated_module text;
ALTER TABLE sessions ADD COLUMN purpose text;
ALTER TABLE sessions DROP CONSTRAINT sessions_client_type_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_type_check CHECK (client_type IN ('web', 'native', 'oauth', 'bot', 'delegated'));
ALTER TABLE sessions ADD CONSTRAINT sessions_delegated_check
    CHECK ((client_type = 'delegated') = (delegated_module IS NOT NULL AND purpose IS NOT NULL));

-- +goose Down
DELETE FROM refresh_tokens USING sessions WHERE refresh_tokens.session_id = sessions.id AND sessions.client_type = 'delegated';
DELETE FROM sessions WHERE client_type = 'delegated';
ALTER TABLE sessions DROP CONSTRAINT sessions_delegated_check;
ALTER TABLE sessions DROP CONSTRAINT sessions_client_type_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_client_type_check CHECK (client_type IN ('web', 'native', 'oauth', 'bot'));
ALTER TABLE sessions DROP COLUMN purpose;
ALTER TABLE sessions DROP COLUMN delegated_module;
