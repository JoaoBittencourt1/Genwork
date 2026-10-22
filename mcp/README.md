# genwork-mcp

MCP server that lets AI assistants (Claude, Claude Code, Cursor, and any other MCP client) manage a Genwork instance as the signed-in user.

The tool list follows the instance. When a session starts, and again on the first request after every 5 minutes, the server reads `GET /api/core/v1/capabilities` and the user's permissions (`GET /api/core/v1/me`). It then exposes only:

- the core tools, plus the tools of each **enabled** module (rule 58), and
- the tools whose permission the user **holds**.

When an administrator enables or disables a module, or changes the user's role, the server adds or removes tools and notifies the client with `tools/list_changed`. The API still checks authorization on every call. The filter keeps assistants from offering actions that would fail.

## Connect an assistant

Every Genwork instance serves MCP over Streamable HTTP at `https://<instance>/mcp`. Nothing is installed on the user's machine: the assistant signs in through the browser and then acts as that user. The web app's **AI assistants** page shows the instance's address and these steps.

- **Claude (claude.ai and Claude Desktop):** Settings → Connectors → Add custom connector, with the MCP address. Claude connects from the internet, so the instance needs a public HTTPS address.
- **Claude Code:** `claude mcp add --transport http genwork https://genwork.example.com/mcp`, then `/mcp` inside Claude Code to sign in.
- **Other MCP clients:** the same address, or this configuration:

  ```json
  { "mcpServers": { "genwork": { "type": "http", "url": "https://genwork.example.com/mcp" } } }
  ```

### How the sign-in works

1. The first request without a token answers `401` with `WWW-Authenticate: Bearer resource_metadata="<instance>/.well-known/oauth-protected-resource/mcp"`.
2. That metadata names the instance itself as the authorization server: core is an OAuth 2.1 server for public clients, with dynamic client registration and PKCE (S256).
3. The client registers, opens `<instance>/oauth/authorize`, and the user approves on Genwork's consent page ("Claude wants to access Genwork").
4. Core issues the usual 15-minute access token and a single-use, rotating refresh token (rule 69). The grant is a session named after the client, ended like any other session; deactivating or removing the member ends it too.

Each request's token is verified with core's public keys and forwarded to the Genwork APIs for that request only, so every authorization decision stays the user's own. An MCP session is bound to the user who opened it; a request with another user's token on it is refused.

### Running the server

The `mcp` service of the stack runs `genwork-mcp serve-http` behind the gateway, which routes `/mcp` and `/.well-known/oauth-protected-resource*` to it. It reads:

| Variable | Use |
| --- | --- |
| `MCP_HTTP_ADDR` | address to listen on, e.g. `:8092` |
| `GENWORK_PUBLIC_URL` | the instance's public address: token issuer and resource |
| `CORE_INTERNAL_URL` | where core's signing keys are fetched |
| `GATEWAY_INTERNAL_URL` | where tool calls reach the Genwork APIs |

## Local server over stdio

`genwork-mcp serve` runs the same tools on stdin/stdout, signed in from the user's machine. Download `genwork-mcp` from a [release](https://github.com/JoaoBittencourt1/Genwork/releases/latest), or build it from a clone:

```sh
make install-mcp        # installs genwork-mcp into $(go env GOPATH)/bin
```

Every stdio command reads `GENWORK_URL`, the address of the instance gateway (the same origin the web app uses):

```sh
export GENWORK_URL=https://genwork.example.com
genwork-mcp login            # asks for e-mail and password
genwork-mcp status           # shows who is signed in
genwork-mcp logout           # revokes the session and forgets it
```

`login` opens a native session in core, which appears as "genwork-mcp on <host>" under the user's sessions. It keeps only the refresh token, in the OS secure store: Keychain, Windows Credential Manager, or Secret Service (rule 69). The access token lives in memory and is renewed automatically.

Refresh tokens are single-use, so every `genwork-mcp` process of the user renews under a shared lock file. Several assistants can therefore run at once without revoking the session.

Claude Code:

```sh
claude mcp add genwork --env GENWORK_URL=https://genwork.example.com -- genwork-mcp serve
```

Claude Desktop, Cursor, and other clients that read a JSON config:

```json
{
  "mcpServers": {
    "genwork": {
      "command": "genwork-mcp",
      "args": ["serve"],
      "env": { "GENWORK_URL": "https://genwork.example.com" }
    }
  }
}
```

`serve` refuses to start, and prints why, when the user is not signed in or the instance is unreachable.

## Tools

Tool names start with the component that owns them. Tools that change data are annotated as such, and deletions are annotated as destructive, so clients can ask before running them.

| Component | Tools |
| --- | --- |
| core | `get_me`, `get_capabilities`, `list_users`, `get_user`, `list_roles`, `list_invites`, `create_invite`, `resend_invite`, `revoke_invite`, `list_sessions`, `revoke_session` |
| gentasks | `list_boards`, `get_board`, `create_board`, `update_board`, `delete_board`, `add_column`, `update_column`, `move_column`, `delete_column`, `list_labels`, `create_label`, `update_label`, `delete_label`, `list_tasks`, `list_my_assigned_tasks`, `get_task`, `create_task`, `update_task`, `move_task`, `archive_task`, `unarchive_task`, `delete_task` |

Installing and enabling modules stays with the installer CLI (rule 64). It is not exposed as tools.

## Adding a module

Create `internal/<module>/` with a `NewToolset(api)` whose `ModuleID` matches the module's id in `module.yaml`. Tool names use the `<module>_` prefix, and each tool names the permission it needs. Then add the toolset to `command.Toolsets`. A module's tools never import another module's package (rule 59).

## Layout

| Path | Role |
| --- | --- |
| `cmd/genwork-mcp/` | wiring only (rule 82) |
| `internal/command/` | `login`, `logout`, `status`, `serve`, `serve-http` |
| `internal/remote/` | Streamable HTTP: bearer verification, protected resource metadata, one server per session |
| `internal/auth/` | sign-in, refresh rotation, secure store, refresh lock |
| `internal/platform/` | configuration and the HTTP client of the gateway |
| `internal/toolset/` | tool definitions and the synchronizer that follows capabilities and permissions |
| `internal/core/`, `internal/gentasks/` | the tools of each component |
| `tests/` | tests, mirroring `internal/` (rule 71); no test reaches the network (rule 25) |
