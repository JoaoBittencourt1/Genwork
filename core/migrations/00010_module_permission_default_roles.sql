-- +goose Up
-- The default roles each module permission last declared, so a role a manifest starts declaring is granted once.
ALTER TABLE permissions ADD COLUMN default_roles text[];

-- Existing module permissions take the roles that hold them now: a grant an admin removed stays removed.
UPDATE permissions
SET default_roles = COALESCE((
    SELECT array_agg(roles.name ORDER BY roles.name)
    FROM role_permissions JOIN roles ON roles.id = role_permissions.role_id
    WHERE role_permissions.permission_id = permissions.id
), '{}')
WHERE module_id IS NOT NULL;

-- +goose Down
ALTER TABLE permissions DROP COLUMN default_roles;
