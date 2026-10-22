## ADDED Requirements

### Requirement: Member permissions
A core migration SHALL seed the permissions `core.member.read` and `core.member.manage` and grant both to the `admin` role only.

#### Scenario: Fresh database
- **WHEN** core migrations run on an empty database
- **THEN** `admin` grants `core.member.read` and `core.member.manage`, and `member` grants neither

### Requirement: List members
Core SHALL expose `GET /api/core/v1/members`, which requires `core.member.read`. It returns `{ "members": [...] }` with every user who has not been removed, ordered by name then e-mail. Each member has `token`, `name`, `email`, `avatarColor`, `role` (`token`, `name`), `status` (`active` or `deactivated`), `joinedAt`, `lastSignInAt` (nullable), and `restrictedModules` (module ids, sorted). No response field may contain a password hash, lockout state, or internal id.

#### Scenario: Admin lists members
- **WHEN** an admin calls the endpoint
- **THEN** core responds `200` with every member who has not been removed, including deactivated members with `status` `deactivated`

#### Scenario: Removed member hidden
- **WHEN** a member has been removed
- **THEN** that member is not in the list

#### Scenario: Missing permission
- **WHEN** a caller without `core.member.read` calls the endpoint
- **THEN** core responds `403` with code `forbidden`

### Requirement: Update a member
Core SHALL expose `PATCH /api/core/v1/members/{token}`, which requires `core.member.manage`. The body MAY contain `role` (a role token), `status` (`active` or `deactivated`), and `restrictedModules` (an array of module ids). An omitted field leaves the stored value unchanged. None of these fields is nullable, so an explicit `null` responds `400` with code `validation_failed` (rule 17). Every present field SHALL be applied in one transaction, and the response is `200` with the member in the list shape.

#### Scenario: Change only the role
- **WHEN** an admin sends `{ "role": "<member role token>" }` for an admin member
- **THEN** core responds `200`, the member's role is `member`, and their status and restricted modules are unchanged

#### Scenario: Explicit null
- **WHEN** the body is `{ "status": null }`
- **THEN** core responds `400` with code `validation_failed` and field `status`, and nothing changes

#### Scenario: Unknown role or status
- **WHEN** `role` is not a known role token, or `status` is neither `active` nor `deactivated`
- **THEN** core responds `400` with code `validation_failed` naming the field (`role: unknown`, `status: invalid`)

### Requirement: Deactivate and reactivate a member
Setting `status` to `deactivated` SHALL record when the member was deactivated and revoke every active session of the member in the same transaction. Setting `status` to `active` SHALL clear it. Deactivation SHALL NOT change the member's role, password, restricted modules, or any data owned by modules. Setting the status the member already has SHALL succeed without changing anything.

#### Scenario: Deactivate
- **WHEN** an admin deactivates a member who has two active sessions
- **THEN** both sessions are revoked, and the member's next core request with their current access token responds `401` with code `invalid_token`

#### Scenario: Reactivate
- **WHEN** an admin sets a deactivated member back to `active`
- **THEN** the member can sign in again with their existing password and keeps their role and restricted modules

### Requirement: Deactivated members cannot sign in
A login with the correct password for a deactivated member SHALL respond `401` with code `invalid_credentials`, the same response and comparable timing as a wrong password (rule 68), and SHALL NOT create a session. Core SHALL treat a session of a deactivated member as inactive, so neither the access token nor refresh works.

#### Scenario: Login while deactivated
- **WHEN** a deactivated member signs in with their correct password
- **THEN** core responds `401` with code `invalid_credentials` and creates no session

#### Scenario: Refresh while deactivated
- **WHEN** a deactivated member presents a refresh token issued before deactivation
- **THEN** core responds `401` and issues no token

### Requirement: Remove a member
Core SHALL expose `DELETE /api/core/v1/members/{token}`, which requires `core.member.manage`. It soft-deletes the member (rule 22), revokes every active session of the member in the same transaction, and responds `204`. A removed member SHALL disappear from the member list and the user directory, and their e-mail SHALL become available for a new invite. Records that modules keep under the removed member's token are not touched.

#### Scenario: Remove
- **WHEN** an admin removes a member
- **THEN** core responds `204`, the member's sessions are revoked, and `GET /api/core/v1/users/{token}` responds `404` with code `user_not_found`

#### Scenario: Invite the same e-mail again
- **WHEN** an admin invites the e-mail of a removed member and the invite is accepted
- **THEN** a new user is created with a new public token

### Requirement: Member management guards
Every member mutation SHALL be authorized in one central place, before any change:
- an unknown or removed member token responds `404` with code `member_not_found`;
- a caller acting on themselves responds `403` with code `cannot_manage_self`;
- a caller whose role does not cover every permission of the target's current role responds `403` with code `member_not_manageable`;
- a new `role` whose permissions the caller's role does not cover responds `403` with code `role_not_assignable`.

#### Scenario: Admin changes their own role
- **WHEN** an admin sends `PATCH /api/core/v1/members/{own token}` or `DELETE` on their own token
- **THEN** core responds `403` with code `cannot_manage_self` and nothing changes

#### Scenario: Unknown member
- **WHEN** the token is unknown or belongs to a removed member
- **THEN** core responds `404` with code `member_not_found`

#### Scenario: Member without permission
- **WHEN** a caller without `core.member.manage` calls `PATCH` or `DELETE`
- **THEN** core responds `403` with code `forbidden`

### Requirement: Roles listing for member management
`GET /api/core/v1/roles` SHALL be available to callers holding either `core.invite.create` or `core.member.manage`, so both the invite form and the member role selector can offer roles.

#### Scenario: Caller with member management only
- **WHEN** a caller holding `core.member.manage` but not `core.invite.create` lists roles
- **THEN** core responds `200` with the roles
