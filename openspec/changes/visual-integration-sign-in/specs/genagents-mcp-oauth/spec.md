## Purpose

Connects bot MCP servers that require OAuth from the web app, so their tokens never have to be pasted into headers.

## ADDED Requirements

### Requirement: Start an authorization
A caller holding `genagents.mcpserver.manage` SHALL start an OAuth authorization for an HTTP MCP server. Genagents SHALL discover the authorization server from the MCP server's protected resource metadata, register a client dynamically unless the admin gave a client id, and return an authorization URL with PKCE (S256), a single-use state, and the `resource` parameter.

#### Scenario: Server with dynamic registration
- **WHEN** an admin clicks "Connect" on an HTTP MCP server whose authorization server supports registration
- **THEN** the response holds an authorization URL on that server, and the browser goes there

#### Scenario: Stdio server
- **WHEN** an admin starts an authorization for a stdio server
- **THEN** the request fails with 400 on `transport`

### Requirement: Callback
`GET /api/genagents/v1/mcp-servers/oauth/callback` SHALL be reachable without a bearer token. It SHALL accept only an unexpired, unused state, exchange the code with the stored verifier, store the tokens encrypted, and redirect to the MCP servers page with the outcome.

#### Scenario: Denied at the provider
- **WHEN** the provider redirects back with `error=access_denied`
- **THEN** the connection reads `error` and the page shows the failure

#### Scenario: Replayed state
- **WHEN** a callback reuses a state already consumed
- **THEN** no token exchange happens and the page shows the failure

### Requirement: Tokens used by runs
When a run claims an OAuth-connected server, genagents SHALL refresh an access token that expires within 5 minutes and add `Authorization: Bearer <token>` to the server's headers for that run only. A failed refresh SHALL mark the connection `error`.

#### Scenario: Connected server in a run
- **WHEN** a bot with a connected MCP server is claimed
- **THEN** the claimed run's server headers carry the bearer token

### Requirement: No token in responses
MCP server responses SHALL show only the connection's status, client id, timestamps, and last error.

#### Scenario: Listing servers
- **WHEN** an admin lists MCP servers after connecting one
- **THEN** no access or refresh token appears in the response

### Requirement: Disconnect
A caller holding `genagents.mcpserver.manage` SHALL disconnect a server, which removes its stored tokens.

#### Scenario: Disconnect
- **WHEN** an admin disconnects a server
- **THEN** its connection reads `disconnected` and runs no longer get the header
