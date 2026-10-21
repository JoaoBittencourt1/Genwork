## Purpose

Gives clients and modules a read-only view of the installation's people so they can pick and reference users by their public token.

## ADDED Requirements

### Requirement: List users
Core SHALL expose `GET /api/core/v1/users`, which requires the `core.user.read` permission. It returns every user who is not deleted, ordered by name, each with `token`, `name`, and `email`, and never with password hashes, lockout state, or role internals. A migration SHALL seed `core.user.read` and grant it to `admin` and `member`.

#### Scenario: Member lists users
- **WHEN** a member calls the endpoint
- **THEN** core responds `200` with every user who is not deleted, ordered by name

#### Scenario: Deleted user hidden
- **WHEN** a user has been soft-deleted
- **THEN** that user is not in the list

#### Scenario: Missing permission
- **WHEN** a role without `core.user.read` calls the endpoint
- **THEN** core responds `403`

### Requirement: Find a user by token
Core SHALL expose `GET /api/core/v1/users/{token}` under the same permission. It returns the same fields as the list, or `404` with code `user_not_found` for an unknown or deleted token.

#### Scenario: Existing user
- **WHEN** the token belongs to a user who is not deleted
- **THEN** core responds `200` with `token`, `name`, and `email`

#### Scenario: Unknown token
- **WHEN** the token is unknown or the user is deleted
- **THEN** core responds `404` with code `user_not_found`
