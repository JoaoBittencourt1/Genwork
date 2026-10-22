-- +goose Up
ALTER TABLE users ADD COLUMN deactivated_at timestamptz;

CREATE TABLE user_module_restrictions (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token      text NOT NULL UNIQUE DEFAULT public_token(),
    user_id    bigint NOT NULL REFERENCES users (id),
    module_id  bigint NOT NULL REFERENCES modules (id),
    created_at timestamptz NOT NULL,
    deleted_at timestamptz
);

CREATE UNIQUE INDEX user_module_restrictions_user_module_key
    ON user_module_restrictions (user_id, module_id) WHERE deleted_at IS NULL;

INSERT INTO permissions (name) VALUES ('core.member.read'), ('core.member.manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles CROSS JOIN permissions
WHERE roles.name = 'admin' AND permissions.name IN ('core.member.read', 'core.member.manage');

-- +goose Down
DELETE FROM role_permissions
USING permissions
WHERE role_permissions.permission_id = permissions.id AND permissions.name IN ('core.member.read', 'core.member.manage');
DELETE FROM permissions WHERE name IN ('core.member.read', 'core.member.manage');
DROP TABLE user_module_restrictions;
ALTER TABLE users DROP COLUMN deactivated_at;
