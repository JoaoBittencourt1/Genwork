## ADDED Requirements

### Requirement: Seeded roles and permissions
Core migrations SHALL create the permissions `core.invite.create`, `core.invite.read`, and `core.invite.revoke`, and the roles `admin` (all core permissions) and `member` (no core permissions). Every user SHALL have exactly one role.

#### Scenario: Fresh database
- **WHEN** core migrations run on an empty database
- **THEN** the `admin` role grants the three invite permissions and the `member` role grants none

### Requirement: Permission enforcement
Every core endpoint that requires a permission SHALL check it against the caller's role on each request, and respond `403` with error code `forbidden` when the permission is missing. Endpoints SHALL declare their permission where the route is registered; no protected route may exist without an explicit declaration.

#### Scenario: Member tries to invite
- **WHEN** a user with the `member` role calls `POST /api/core/v1/invites`
- **THEN** core responds `403` with error code `forbidden`

#### Scenario: Unauthenticated request to a protected endpoint
- **WHEN** a request without a valid access token calls a protected endpoint
- **THEN** core responds `401` with error code `invalid_token`

### Requirement: List roles
`GET /api/core/v1/roles` SHALL return each role's public token, name, and permission names to callers holding `core.invite.create`, so the invite form can offer them.

#### Scenario: Admin lists roles
- **WHEN** an admin calls `GET /api/core/v1/roles`
- **THEN** core responds `200` with `admin` and `member` and their permissions

### Requirement: No privilege escalation through roles
A user SHALL only assign, through any flow in this change, a role whose permissions are a subset of their own role's permissions.

#### Scenario: Inviting with a more powerful role
- **WHEN** a user whose role lacks some permission of the `admin` role invites someone as `admin`
- **THEN** core responds `403` with error code `role_not_assignable`
