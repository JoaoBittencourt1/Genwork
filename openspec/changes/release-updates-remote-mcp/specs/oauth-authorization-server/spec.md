## Purpose

Makes core the OAuth 2.1 authorization server for AI assistants, so a person connects Claude or another MCP client to Genwork by signing in through the browser, without installing anything.

## ADDED Requirements

### Requirement: Authorization server metadata
Core SHALL serve RFC 8414 metadata at `GET /.well-known/oauth-authorization-server` without authentication. The `issuer` SHALL be the instance's public URL. The metadata SHALL include:
- `authorization_endpoint`: `<public URL>/oauth/authorize`;
- `token_endpoint`: `<public URL>/api/core/v1/oauth/token`;
- `registration_endpoint`: `<public URL>/api/core/v1/oauth/register`;
- `response_types_supported`: `["code"]`;
- `grant_types_supported`: `["authorization_code", "refresh_token"]`;
- `code_challenge_methods_supported`: `["S256"]`;
- `token_endpoint_auth_methods_supported`: `["none"]`;
- `scopes_supported`: `["genwork"]`.

#### Scenario: Client discovers the server
- **WHEN** a client requests `/.well-known/oauth-authorization-server` through the gateway
- **THEN** it receives `200` with the metadata above, built from the public URL

### Requirement: Dynamic client registration
`POST /api/core/v1/oauth/register` SHALL register a public client per RFC 7591 without authentication, rate limited per client address. It SHALL accept `client_name` (1 to 100 characters, default "MCP client") and `redirect_uris` (1 to 5 entries). Each redirect URI SHALL use `https`, or `http` on a loopback host (`localhost`, `127.0.0.1`, or `[::1]`) with any port. It SHALL answer `201` with a `client_id` (the client's public token), `client_id_issued_at`, the registered values, and `token_endpoint_auth_method: "none"`. It SHALL never issue a client secret.

#### Scenario: Claude registers
- **WHEN** a client registers `client_name` "Claude" with `redirect_uris` `["https://claude.ai/api/mcp/auth_callback"]`
- **THEN** core answers `201` with a new `client_id` and no `client_secret`

#### Scenario: Local client registers
- **WHEN** a client registers `http://127.0.0.1:53682/callback`
- **THEN** the registration succeeds

#### Scenario: Insecure redirect
- **WHEN** a client registers `http://example.com/callback`
- **THEN** core answers `400` with `error: "invalid_redirect_uri"`

### Requirement: Authorization request validation
Core SHALL validate an authorization request before showing consent:
- If the client is unknown, or the `redirect_uri` is not one of its registered URIs, core SHALL report the error to the user and SHALL NOT redirect.
- Otherwise, these errors SHALL redirect to the `redirect_uri` with `error` and the original `state`:
  - `response_type` other than `code`: `unsupported_response_type`;
  - a missing `code_challenge` or a `code_challenge_method` other than `S256`: `invalid_request`;
  - a `scope` other than `genwork`: `invalid_scope`;
  - a `resource` other than the instance's MCP resource URL: `invalid_target`.

#### Scenario: Unregistered redirect URI
- **WHEN** the consent page asks core about a request whose `redirect_uri` the client never registered
- **THEN** core answers `400 invalid_redirect_uri` and the page shows an error without leaving Genwork

#### Scenario: Missing PKCE
- **WHEN** a request has no `code_challenge`
- **THEN** the user is sent to the redirect URI with `error=invalid_request` and the request's `state`

### Requirement: Consent issues an authorization code
A signed-in, active user SHALL approve or deny a valid request with `POST /api/core/v1/oauth/authorizations`. Core SHALL answer with the URL to send the browser to:
- **approve**: the redirect URI with `code`, `state`, and `iss`;
- **deny**: the redirect URI with `error=access_denied` and `state`.

Codes SHALL have at least 128 bits of entropy and be stored only as hashes (rule 67). Each code SHALL be single-use, expire after 5 minutes, and be bound to the client, the redirect URI, the user, and the PKCE challenge.

#### Scenario: User approves
- **WHEN** an active user approves a valid request
- **THEN** core answers with the redirect URI carrying a new `code`, the original `state`, and `iss` equal to the public URL

#### Scenario: User denies
- **WHEN** the user denies
- **THEN** core answers with the redirect URI carrying `error=access_denied` and the original `state`, and no code is created

### Requirement: Token endpoint
`POST /api/core/v1/oauth/token` SHALL accept `application/x-www-form-urlencoded` requests without client authentication, rate limited per client address. It SHALL answer with `Cache-Control: no-store`, and errors SHALL use the RFC 6749 JSON format (`error`, `error_description`).
- **`authorization_code` grant:** requires `code`, `redirect_uri`, `client_id`, and `code_verifier`. It SHALL succeed only for an unexpired, unused code issued to that client with that redirect URI and a verifier matching the S256 challenge.
- **`refresh_token` grant:** requires `refresh_token` and `client_id`. It SHALL rotate the refresh token of a session belonging to that client.

Successful answers SHALL contain:
- `access_token`: a core access token in the same format as every other session (audience `genwork`, the user's effective permissions, at most 15 minutes);
- `token_type: "Bearer"`;
- `expires_in`;
- `refresh_token`: a single-use refresh token;
- `scope: "genwork"`.

#### Scenario: Code exchange
- **WHEN** a client exchanges a valid code with the matching verifier
- **THEN** core answers `200` with an access token that core and every module accept, and a refresh token

#### Scenario: Wrong verifier
- **WHEN** the `code_verifier` does not match the challenge
- **THEN** core answers `400` with `error: "invalid_grant"`

#### Scenario: Code reused
- **WHEN** a code that was already exchanged is presented again
- **THEN** core answers `400 invalid_grant` and revokes the session created by the first exchange

#### Scenario: Refresh rotation
- **WHEN** a client presents its current refresh token
- **THEN** core answers with a new access token and a new refresh token, and the presented one stops working

#### Scenario: Refresh token reused
- **WHEN** a refresh token that was already rotated is presented again
- **THEN** core answers `400 invalid_grant` and revokes the whole session (rule 69)

#### Scenario: Refresh by another client
- **WHEN** a client presents a refresh token issued to a different client
- **THEN** core answers `400 invalid_grant` and nothing rotates

### Requirement: Assistant grants are sessions
Each successful code exchange SHALL start a session with client type `oauth`, named after the client. These sessions SHALL appear in the user's session list and be revocable there. Revoking the session, deactivating the member, or removing the member SHALL make its refresh token fail.

#### Scenario: User revokes Claude
- **WHEN** the user revokes the session named "Claude"
- **THEN** the next refresh by that client fails with `invalid_grant`, and the client has to ask for consent again
