## Purpose

Defines how modules trust core-issued access tokens and authorize requests locally, without storing credentials or calling core on every request (rule 57).

## ADDED Requirements

### Requirement: Access tokens carry permissions
Every access token core issues SHALL include a `permissions` claim listing the permission names granted to the user's role when the token was issued. It also keeps `sub` (user token), issuer, audience, expiry (at most 15 minutes), and `kid`.

#### Scenario: Member token
- **WHEN** a member signs in after gentasks has registered
- **THEN** the access token's `permissions` claim contains every `gentasks.*` permission granted to `member`

#### Scenario: Grants change
- **WHEN** a role loses a permission
- **THEN** access tokens issued afterwards no longer list it, and tokens issued earlier keep it until they expire

### Requirement: Modules validate tokens with core's public keys
A module SHALL accept a request only when its `Authorization: Bearer` access token has a valid EdDSA signature from a key in core's JWKS, the expected issuer and audience, and has not expired. The module SHALL fetch the JWKS from core, cache it, and refetch it when it sees an unknown `kid`, at most once per minute.

#### Scenario: Valid token
- **WHEN** a request carries an unexpired token signed by core
- **THEN** the module processes it as the token's user

#### Scenario: Missing, expired, or forged token
- **WHEN** the token is missing, expired, has the wrong issuer or audience, or is signed by a key not in the JWKS
- **THEN** the module responds `401` with code `invalid_token`

#### Scenario: Core unreachable when the key is unknown
- **WHEN** the token's `kid` is not cached and the JWKS cannot be fetched
- **THEN** the module responds `503`, not `401`

### Requirement: Modules authorize from the permissions claim
Every module route SHALL declare a policy (`authenticated` or a required permission) when it is registered, and a route without a policy MUST fail at startup (rule 18). A route that requires a permission SHALL respond `403` with code `forbidden` when the token's `permissions` claim does not contain it.

#### Scenario: Missing permission
- **WHEN** a token without `gentasks.task.delete` calls `DELETE /api/gentasks/v1/tasks/{token}`
- **THEN** gentasks responds `403`

#### Scenario: Route registered without a policy
- **WHEN** module code registers a route with no policy
- **THEN** the module refuses to start
