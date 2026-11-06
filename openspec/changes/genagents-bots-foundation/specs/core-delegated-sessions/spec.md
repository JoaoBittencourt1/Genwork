## Purpose

Lets a signed-in user grant a module a bounded, revocable session that acts as that user, so a module can perform work on the user's behalf after the user's own short-lived access token has expired.

## ADDED Requirements

### Requirement: Grant a delegated session
A person signed in on the web or a native app SHALL be able to grant a delegated session to an enabled module, with a purpose label and a lifetime of at most 24 hours idle and 7 days absolute. Core SHALL return a refresh token for that session once, to the module calling on the user's behalf with the user's access token. The session SHALL carry exactly the user's own permissions at each refresh, so a role change applies at the next refresh. A bot, OAuth (assistant), or delegated session SHALL NOT be able to grant one (`403 forbidden`), so a short-lived or limited token never becomes a renewable one. The module name is a label for the session list: the token is a normal bearer token and is not limited to that module's APIs.

#### Scenario: Grant
- **WHEN** an admin, through genagents, grants a delegated session with purpose "Admin chat"
- **THEN** core creates a session named after the module and purpose and returns its refresh token once

#### Scenario: Disabled module
- **WHEN** a grant names a module that is not enabled
- **THEN** the request is rejected with 400

### Requirement: Delegated sessions are ordinary sessions
A delegated session SHALL follow the session rules for refresh rotation and reuse detection (rule 69). It SHALL be listed among the user's sessions with its module and purpose, and SHALL be ended like any other session: by the user, by an admin managing that member, by deactivating or removing the member, or by the module revoking it.

#### Scenario: User ends the session
- **WHEN** the user ends the "Admin chat" session from their session list
- **THEN** the next refresh with its refresh token fails

#### Scenario: Refresh token reuse
- **WHEN** an already-used refresh token of a delegated session is presented
- **THEN** the whole session is revoked

#### Scenario: Role lowered
- **WHEN** the user's role changes from `admin` to `member` while a delegated session is active
- **THEN** the next access token issued for that session carries `member` permissions
