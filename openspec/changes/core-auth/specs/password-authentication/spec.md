## ADDED Requirements

### Requirement: Password hashing
Core SHALL store passwords only as Argon2id hashes in PHC string format, with a unique 16-byte random salt per hash and parameters of at least 64 MiB memory, 3 iterations, and parallelism 2. Passwords SHALL be between 8 and 128 characters (counted as Unicode code points), with no composition rules.

#### Scenario: Password is never stored in clear
- **WHEN** a user account is created with a valid password
- **THEN** the stored value is an `$argon2id$` PHC string and the plain password is not persisted or logged anywhere

#### Scenario: Password too short or too long
- **WHEN** an account is created with a password of 7 or 129 characters
- **THEN** the request is rejected with `400` and error code `password_length_invalid`

#### Scenario: Outdated parameters are upgraded on login
- **WHEN** a user signs in successfully and their stored hash uses parameters below the current ones
- **THEN** core rehashes the password with the current parameters and replaces the stored hash

### Requirement: Login
`POST /api/core/v1/auth/login` SHALL accept `email`, `password`, `client` (`"web"` or `"native"`), and an optional `deviceName`. E-mail matching SHALL be case-insensitive after trimming. On success it SHALL create a session and return an access token and its expiry; the refresh token SHALL be returned in the response body for `native` clients and set as a cookie for `web` clients.

#### Scenario: Successful native login
- **WHEN** an active user posts correct credentials with `client: "native"`
- **THEN** core responds `200` with `accessToken`, `accessTokenExpiresAt`, `refreshToken`, and `session` (its public token), and updates the user's `last_login_at`

#### Scenario: Successful web login
- **WHEN** an active user posts correct credentials with `client: "web"`
- **THEN** core responds `200` with `accessToken` and `accessTokenExpiresAt`, sets the refresh token in a cookie named `genwork_refresh` with `HttpOnly`, `Secure`, `SameSite=Strict`, and `Path=/api/core/v1/auth`, and does not include the refresh token in the body

#### Scenario: Unknown e-mail and wrong password are indistinguishable
- **WHEN** a login uses an e-mail with no account, or a valid e-mail with a wrong password
- **THEN** both respond `401` with error code `invalid_credentials`, and core performs an Argon2id verification in both cases so response times are comparable

### Requirement: Access token
Access tokens SHALL be JWTs signed with EdDSA (Ed25519) using the private key from configuration, valid for 15 minutes, carrying `iss` (the instance public URL), `aud` (`"genwork"`), `sub` (user public token), `sid` (session public token), `iat`, and `exp`. Core SHALL publish the public key as a JWK Set at `GET /api/core/v1/auth/jwks`.

#### Scenario: Protected endpoint with a valid token
- **WHEN** a request carries `Authorization: Bearer <access token>` that is correctly signed, unexpired, and whose session is not revoked
- **THEN** core processes the request as that user

#### Scenario: Expired or tampered token
- **WHEN** a request carries an expired access token, or one with an invalid signature, issuer, or audience
- **THEN** core responds `401` with error code `invalid_token`

#### Scenario: Public key is published
- **WHEN** a client requests `GET /api/core/v1/auth/jwks`
- **THEN** core responds `200` with a JWK Set containing the Ed25519 public key and its `kid`

### Requirement: Refresh token rotation
`POST /api/core/v1/auth/refresh` SHALL accept the refresh token (from the `genwork_refresh` cookie for web, or the request body for native), and on success SHALL return a new access token and a new refresh token, marking the presented one as used. Refresh tokens SHALL be 32 random bytes encoded URL-safe, stored only as SHA-256 hashes, and compared in constant time. A session SHALL expire after 30 days without a refresh, and in any case 90 days after it was created.

#### Scenario: Successful refresh
- **WHEN** a client presents an unused, unexpired refresh token of an active session
- **THEN** core responds `200` with a new access token and a new refresh token (delivered the same way as on login), and the presented token can no longer be used

#### Scenario: Reuse of an already-used refresh token
- **WHEN** a client presents a refresh token that was already used
- **THEN** core revokes the whole session and responds `401` with error code `session_revoked`

#### Scenario: Expired session
- **WHEN** a client presents a refresh token whose session passed its idle or absolute lifetime
- **THEN** core responds `401` with error code `session_expired`

### Requirement: Logout and sessions
An authenticated user SHALL be able to end the current session with `POST /api/core/v1/auth/logout`, list their active sessions with `GET /api/core/v1/auth/sessions` (public token, client type, device name, created at, last used at, and whether it is the current one), and revoke one of them with `DELETE /api/core/v1/auth/sessions/{token}`. A revoked session SHALL stop being accepted immediately, including access tokens already issued for it.

#### Scenario: Logout on web
- **WHEN** a web user calls logout
- **THEN** core revokes the session, responds `204`, and clears the `genwork_refresh` cookie

#### Scenario: Revoke another device
- **WHEN** a user deletes one of their own sessions by token
- **THEN** core responds `204` and any request using that session's access or refresh token fails with `401`

#### Scenario: Cannot revoke someone else's session
- **WHEN** a user tries to delete a session token that belongs to another user
- **THEN** core responds `404`

### Requirement: Current user
`GET /api/core/v1/me` SHALL return the authenticated user's public token, e-mail, name, role (token and name), and the list of permission names the role grants.

#### Scenario: Fetch current user
- **WHEN** an authenticated user calls `GET /api/core/v1/me`
- **THEN** core responds `200` with their token, e-mail, name, role, and permissions, and never with internal ids or the password hash

### Requirement: Rate limiting and lockout
Login, refresh, and invitation-acceptance endpoints SHALL be rate limited per client address, where the client address is the TCP peer address unless it belongs to a configured trusted proxy, in which case it is the right-most untrusted address in `X-Forwarded-For`. After 10 consecutive failed logins, an account SHALL be locked for 15 minutes; a successful login resets the counter.

#### Scenario: Too many requests from one address
- **WHEN** one client address exceeds the limit on an auth endpoint
- **THEN** core responds `429` with error code `rate_limited` and a `Retry-After` header

#### Scenario: Spoofed forwarding header is ignored
- **WHEN** a request arrives directly (not from a trusted proxy) with an `X-Forwarded-For` header
- **THEN** core rate limits by the TCP peer address and ignores the header

#### Scenario: Locked account
- **WHEN** an account has 10 consecutive failed logins and a new login arrives within 15 minutes, even with the correct password
- **THEN** core responds `401` with error code `invalid_credentials`, the same response as a wrong password

### Requirement: Machine-readable errors
Every error response from core SHALL be JSON of the form `{"error": {"code": "<snake_case_code>"}}`, optionally with a `fields` object for validation errors. Responses SHALL NOT contain user-facing sentences; clients translate codes.

#### Scenario: Validation error shape
- **WHEN** a login request has a malformed e-mail
- **THEN** core responds `400` with `{"error": {"code": "validation_failed", "fields": {"email": "invalid"}}}`
