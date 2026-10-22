-- +goose Up
INSERT INTO permissions (name) VALUES ('core.system.update');

INSERT INTO role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM roles CROSS JOIN permissions
WHERE roles.name = 'admin' AND permissions.name = 'core.system.update';

-- +goose Down
DELETE FROM role_permissions
USING permissions
WHERE role_permissions.permission_id = permissions.id AND permissions.name = 'core.system.update';
DELETE FROM permissions WHERE name = 'core.system.update';
