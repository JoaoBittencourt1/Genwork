## Purpose

Lets the installation have non-human members (bots) that hold a role, appear wherever members appear, and authenticate without a password, so modules can give automated actors their own identity and permissions.

## ADDED Requirements

### Requirement: Member kind
Every member SHALL have a kind, `human` or `bot`. Existing members SHALL be `human`. A bot member SHALL have a name, a role, and no e-mail address or password, and SHALL be created only through the bot member API, never through invitations or sign-up.

#### Scenario: Existing members after upgrade
- **WHEN** an installation with members is upgraded
- **THEN** every existing member has kind `human` and signs in exactly as before

#### Scenario: Bot cannot sign in with a password
- **WHEN** a password login is attempted for a bot member's name
- **THEN** the response is the generic invalid-credentials error used for any failed login (rule 68)

### Requirement: Bot member management
A caller holding `core.member.manage` SHALL be able to create a bot member (name and role), rename it, change its role, and deactivate or remove it, with the same soft-delete and deactivation rules as human members. Creating a bot member SHALL return its token and a client secret **once**; core SHALL store only a hash of the secret (rule 67). The caller SHALL be able to rotate the secret, which invalidates the previous one and revokes every session of the bot immediately.

#### Scenario: Create a bot member
- **WHEN** an admin's request creates bot member "Dev Bot" with role `member`
- **THEN** the response carries the bot member token and a client secret, and later reads never return the secret

#### Scenario: Rotate the secret
- **WHEN** the secret of a bot member is rotated
- **THEN** a token request with the previous secret fails, one with the new secret succeeds, and access tokens issued before the rotation stop working

#### Scenario: Member without permission
- **WHEN** a caller without `core.member.manage` tries to create a bot member
- **THEN** the request is rejected with 403

### Requirement: Bot member authentication
Core's OAuth token endpoint SHALL accept the `client_credentials` grant with a bot member token as client id and its secret, and SHALL issue the usual short-lived access token (rule 69) whose subject is the bot member, carrying the bot's role permissions. No refresh token SHALL be issued for this grant. A deactivated or removed bot member SHALL NOT obtain tokens, and access tokens SHALL expire within 15 minutes of issue. Token requests SHALL be rate limited per client address and bot member, so a third party cannot exhaust a bot's requests by guessing its public token.

#### Scenario: Token for an active bot
- **WHEN** an active bot member requests a token with a valid secret
- **THEN** it receives an access token valid for at most 15 minutes and no refresh token

#### Scenario: Deactivated bot
- **WHEN** a deactivated bot member requests a token
- **THEN** the request fails with the same error as a wrong secret

### Requirement: Bots in member and directory reads
Member lists, the user directory, and the `me` endpoint SHALL include the member kind. Member lists and the directory SHALL include active bot members alongside humans so modules can show bots in pickers (for example as task assignees). Seat-like counts shown to admins SHALL report humans and bots separately.

#### Scenario: Directory lists a bot
- **WHEN** a member reads the user directory while bot member "Dev Bot" is active
- **THEN** "Dev Bot" is listed with kind `bot`

#### Scenario: Bot reads itself
- **WHEN** a bot member calls `me` with its access token
- **THEN** the response has kind `bot`, its role, and its permissions
