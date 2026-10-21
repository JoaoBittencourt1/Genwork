-- +goose Up
-- Serialized because concurrent CREATE EXTENSION calls in one database fail with a unique violation.
SELECT pg_advisory_xact_lock(4805001);
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;

-- +goose StatementBegin
CREATE FUNCTION public_token() RETURNS text LANGUAGE sql VOLATILE AS $$
  SELECT rtrim(translate(encode(public.gen_random_bytes(16), 'base64'), '+/', '-_'), '=')
$$;
-- +goose StatementEnd

CREATE TABLE permissions (
    id    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token text NOT NULL UNIQUE DEFAULT public_token(),
    name  text NOT NULL UNIQUE
);

CREATE TABLE roles (
    id    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token text NOT NULL UNIQUE DEFAULT public_token(),
    name  text NOT NULL UNIQUE
);

CREATE TABLE role_permissions (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token         text NOT NULL UNIQUE DEFAULT public_token(),
    role_id       bigint NOT NULL REFERENCES roles (id),
    permission_id bigint NOT NULL REFERENCES permissions (id),
    UNIQUE (role_id, permission_id)
);

INSERT INTO permissions (name) VALUES
    ('core.invite.create'),
    ('core.invite.read'),
    ('core.invite.revoke');

INSERT INTO roles (name) VALUES ('admin'), ('member');

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles CROSS JOIN permissions
WHERE roles.name = 'admin';

-- +goose Down
DROP TABLE role_permissions;
DROP TABLE roles;
DROP TABLE permissions;
DROP FUNCTION public_token();
