## ADDED Requirements

### Requirement: Restricted modules per member
A member SHALL have access to every module by default. `restrictedModules` in `PATCH /api/core/v1/members/{token}` SHALL replace the member's whole set of restricted modules. Each id SHALL name a registered module, enabled or not, or the request responds `400` with code `validation_failed` and field `restrictedModules: unknown_module`. Duplicate ids SHALL count once, and an empty array SHALL lift every restriction. Restrictions SHALL survive disabling and re-enabling the module, and deactivating and reactivating the member.

#### Scenario: Restrict a member from a module
- **WHEN** an admin sends `{ "restrictedModules": ["gentasks"] }`
- **THEN** core responds `200` and the member's `restrictedModules` is `["gentasks"]`

#### Scenario: Lift every restriction
- **WHEN** an admin sends `{ "restrictedModules": [] }` for a restricted member
- **THEN** the member's `restrictedModules` is empty

#### Scenario: Unknown module
- **WHEN** `restrictedModules` contains an id no module ever registered
- **THEN** core responds `400` with code `validation_failed` and nothing changes

### Requirement: Effective permissions
A member's effective permissions SHALL be their role's permissions minus every permission declared by a module restricted for them. Core permissions SHALL never be affected by module restrictions. Every access token core issues, at sign-in, invite acceptance, and refresh, SHALL carry the effective permissions, and `GET /api/core/v1/me` SHALL return them.

#### Scenario: Restricted member refreshes
- **WHEN** a member restricted from `gentasks` refreshes their session
- **THEN** the new access token and `/me` contain no `gentasks.*` permission and still contain their core permissions

#### Scenario: Restriction lifted
- **WHEN** the restriction is lifted and the member refreshes
- **THEN** the new access token contains the `gentasks.*` permissions their role grants

### Requirement: Capabilities follow the caller's access
`GET /api/core/v1/capabilities` SHALL list only the enabled modules the caller is not restricted from, so clients hide restricted modules (rule 58).

#### Scenario: Restricted module hidden
- **WHEN** a member restricted from `gentasks` reads capabilities while `gentasks` is enabled
- **THEN** `gentasks` is not in `modules`

#### Scenario: Unrestricted member
- **WHEN** a member with no restriction reads capabilities
- **THEN** `modules` lists every enabled module

### Requirement: Restrictions apply from the next access token
A restriction SHALL apply in modules from the member's next access token. An access token issued before the change stays valid in modules until it expires. Core SHALL NOT revoke sessions because of a restriction change.

#### Scenario: Token issued before the restriction
- **WHEN** an admin restricts a member from `gentasks` while the member holds an access token issued before the restriction
- **THEN** gentasks keeps accepting that token until it expires, and the member's next refresh yields a token without `gentasks.*`
