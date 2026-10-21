-- +goose Up
INSERT INTO permissions (name) VALUES ('core.user.read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles CROSS JOIN permissions
WHERE roles.name IN ('admin', 'member') AND permissions.name = 'core.user.read';

-- +goose Down
DELETE FROM role_permissions
USING permissions
WHERE role_permissions.permission_id = permissions.id AND permissions.name = 'core.user.read';
DELETE FROM permissions WHERE name = 'core.user.read';
