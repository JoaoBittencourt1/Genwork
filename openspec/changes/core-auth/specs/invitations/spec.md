## ADDED Requirements

### Requirement: Create invite
A user holding `core.invite.create` SHALL be able to invite an e-mail address with a role through `POST /api/core/v1/invites` (`email`, `role` token). Core SHALL generate a 32-byte random secret, store only its SHA-256 hash, set expiry to 7 days, send the invite e-mail, and respond `201` with the invite (public token, e-mail, role, inviter, expiry), the `link` (`<public URL>/invite/<secret>`), and `emailSent` (whether SMTP delivery succeeded).

#### Scenario: Successful invite
- **WHEN** an admin invites `bruno@example.com` as `member`
- **THEN** core responds `201` with the link and `emailSent: true`, and the e-mail with that link arrives in the SMTP server

#### Scenario: E-mail delivery fails
- **WHEN** the SMTP server is unreachable while creating an invite
- **THEN** the invite is still created, core responds `201` with the link and `emailSent: false`, and logs the delivery failure without the secret

#### Scenario: E-mail already has an account
- **WHEN** the invited e-mail belongs to an existing user
- **THEN** core responds `409` with error code `email_already_registered`

#### Scenario: E-mail already has a pending invite
- **WHEN** the invited e-mail already has an unexpired, unaccepted, unrevoked invite
- **THEN** core responds `409` with error code `invite_already_pending`

### Requirement: Invite status is derived
An invite's status SHALL be derived from its timestamps, never stored separately: `accepted` if `accepted_at` is set, `revoked` if `revoked_at` is set, `expired` if `expires_at` is in the past, otherwise `pending`.

#### Scenario: Invite passes its expiry
- **WHEN** an invite created 7 days and 1 minute ago was never accepted or revoked
- **THEN** it is reported as `expired` and cannot be accepted

### Requirement: List, revoke, and resend invites
Users holding `core.invite.read` SHALL list invites with `GET /api/core/v1/invites` (optionally filtered by status). Users holding `core.invite.revoke` SHALL revoke a pending invite with `DELETE /api/core/v1/invites/{token}`. Users holding `core.invite.create` SHALL resend a pending or expired invite with `POST /api/core/v1/invites/{token}/resend`, which replaces the secret, resets expiry to 7 days, sends the e-mail again, and returns the new link; the previous link stops working.

#### Scenario: Revoke
- **WHEN** an admin revokes a pending invite
- **THEN** core responds `204` and the invite link returns `invite_not_found` when opened

#### Scenario: Resend invalidates the previous link
- **WHEN** an admin resends an invite
- **THEN** core responds `200` with a new link, and the previous link returns `invite_not_found`

#### Scenario: Cannot revoke an accepted invite
- **WHEN** an admin revokes an invite that was already accepted
- **THEN** core responds `409` with error code `invite_not_pending`

### Requirement: Inspect and accept an invite
Anyone holding the link SHALL be able to read the invite with `GET /api/core/v1/invitations/{secret}` (instance name, invited e-mail, expiry) and accept it with `POST /api/core/v1/invitations/{secret}/accept` (`name`, `password`, `client`, optional `deviceName`). Accepting SHALL, in one transaction, create an active user with the invited e-mail and role, mark the invite accepted, and sign the user in exactly as a successful login does.

#### Scenario: Open a valid link
- **WHEN** someone requests a pending invite by its secret
- **THEN** core responds `200` with the instance name, invited e-mail, and expiry

#### Scenario: Accept a valid invite
- **WHEN** someone accepts a pending invite with a name and a valid password
- **THEN** the user is created with the invite's e-mail and role, the invite becomes `accepted`, and the response matches a successful login for the given client type

#### Scenario: Invalid, used, revoked, or expired link
- **WHEN** someone opens or accepts a secret that does not match a pending invite
- **THEN** core responds `404` with error code `invite_not_found`, without revealing which case applies

#### Scenario: Concurrent acceptance
- **WHEN** two requests accept the same invite at the same time
- **THEN** exactly one succeeds and the other responds `404` with `invite_not_found`

### Requirement: Invite e-mail
The invite e-mail SHALL be sent in the instance's default locale (`pt-BR` or `en`, from configuration), in both plain-text and HTML, containing the instance name, the inviter's name, the link, and the expiry date. Its copy SHALL come from localized templates, not from code.

#### Scenario: Default locale pt-BR
- **WHEN** the instance default locale is `pt-BR` and an invite is created
- **THEN** the e-mail captured by Mailpit is in Portuguese and contains the link and expiry
