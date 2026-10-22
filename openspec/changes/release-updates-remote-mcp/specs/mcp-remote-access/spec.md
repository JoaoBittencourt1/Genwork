## Purpose

Exposes Genwork's MCP tools over Streamable HTTP at the instance's own address, so assistants connect with a URL and a browser sign-in instead of a locally installed binary.

## ADDED Requirements

### Requirement: MCP endpoint behind the gateway
The gateway SHALL route `/mcp` to the HTTP MCP server, which SHALL speak the MCP Streamable HTTP transport. The MCP server SHALL have no other public entrypoint.

#### Scenario: Client initializes a session
- **WHEN** an MCP client sends `initialize` to `<public URL>/mcp` with a valid access token
- **THEN** the server answers with its capabilities and a session id

### Requirement: Bearer authentication with discovery
Every request to `/mcp` SHALL carry a core access token in the `Authorization: Bearer` header. The MCP server SHALL verify it with core's public keys, issuer, and audience. When the token is missing, invalid, or expired, the server SHALL answer `401` with `WWW-Authenticate: Bearer resource_metadata="<public URL>/.well-known/oauth-protected-resource/mcp"`.

#### Scenario: No token
- **WHEN** a client calls `/mcp` without `Authorization`
- **THEN** the server answers `401` with the `WWW-Authenticate` header pointing to the protected resource metadata

#### Scenario: Expired token
- **WHEN** a client calls `/mcp` with an access token past its expiry
- **THEN** the server answers `401` with the same header, and the client refreshes and retries

### Requirement: Protected resource metadata
The MCP server SHALL serve RFC 9728 metadata, without authentication, at both `/.well-known/oauth-protected-resource/mcp` and `/.well-known/oauth-protected-resource`. The metadata SHALL contain:
- `resource`: `<public URL>/mcp`;
- `authorization_servers`: `["<public URL>"]`;
- `bearer_methods_supported`: `["header"]`;
- `scopes_supported`: `["genwork"]`.

#### Scenario: Client discovers the authorization server
- **WHEN** a client requests `/.well-known/oauth-protected-resource/mcp`
- **THEN** it receives the metadata naming the instance's public URL as authorization server

### Requirement: Tools follow the signed-in user
The tools listed in a session SHALL follow the same rule as the stdio server: core tools plus the tools of each enabled module, limited to the permissions the token's user holds. The server SHALL re-read capabilities and permissions at most every 5 minutes while the session is in use, and SHALL notify the client with `tools/list_changed` when the list changes.

#### Scenario: Member without gentasks permissions
- **WHEN** a member whose role lacks `gentasks.board.manage` lists tools
- **THEN** `gentasks_create_board` is not in the list

#### Scenario: Module disabled during a session
- **WHEN** an admin disables gentasks and the session's next request comes more than 5 minutes after the last sync
- **THEN** the gentasks tools disappear and the client receives `tools/list_changed`

### Requirement: Calls act with the request's token
Each tool call SHALL call the Genwork APIs with the access token of the request that carried it. The MCP server SHALL NOT store tokens beyond the request. A session SHALL be bound to the user who created it: a request on that session with a token of another user SHALL be refused.

#### Scenario: Token refreshed mid-session
- **WHEN** a client refreshes its access token and continues the same MCP session
- **THEN** the next tool call uses the new token and succeeds

#### Scenario: Session hijack attempt
- **WHEN** a request presents user B's token with a session id created by user A
- **THEN** the server refuses the request and runs no tool

### Requirement: Stdio server unchanged
`genwork-mcp serve` over stdio, `login`, `logout`, and `status` SHALL keep working as before. The binaries SHALL be published in every release.

#### Scenario: Existing stdio setup
- **WHEN** a user who configured `genwork-mcp serve` before this change starts their assistant
- **THEN** the tools work as before
