# genwork-mcp

MCP server that lets AI assistants (Claude Code, Claude Desktop, Cursor, and any other MCP client) manage a Genwork instance as the signed-in user.

The tool list follows the instance. At startup, and every 5 minutes after that, the server reads `GET /api/core/v1/capabilities` and the user's permissions (`GET /api/core/v1/me`). It then exposes only:

- the core tools, plus the tools of each **enabled** module (rule 58), and
- the tools whose permission the user **holds**.

When an administrator enables or disables a module, or changes the user's role, the server adds or removes tools and notifies the client with `tools/list_changed`. The API still checks authorization on every call. The filter keeps assistants from offering actions that would fail.

## Install

```sh
make install-mcp        # installs genwork-mcp into $(go env GOPATH)/bin
```

## Sign in

Every command reads `GENWORK_URL`, the address of the instance gateway (the same origin the web app uses):

```sh
export GENWORK_URL=https://genwork.example.com
genwork-mcp login            # asks for e-mail and password
genwork-mcp status           # shows who is signed in
genwork-mcp logout           # revokes the session and forgets it
```

`login` opens a native session in core, which appears as "genwork-mcp on <host>" under the user's sessions. It keeps only the refresh token, in the OS secure store: Keychain, Windows Credential Manager, or Secret Service (rule 69). The access token lives in memory and is renewed automatically.

Refresh tokens are single-use, so every `genwork-mcp` process of the user renews under a shared lock file. Several assistants can therefore run at once without revoking the session.

## Connect an assistant

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
| `internal/command/` | `login`, `logout`, `status`, `serve` |
| `internal/auth/` | sign-in, refresh rotation, secure store, refresh lock |
| `internal/platform/` | configuration and the HTTP client of the gateway |
| `internal/toolset/` | tool definitions and the synchronizer that follows capabilities and permissions |
| `internal/core/`, `internal/gentasks/` | the tools of each component |
| `tests/` | tests, mirroring `internal/` (rule 71); no test reaches the network (rule 25) |

## Next

Remote access on the gateway: Streamable HTTP at `/mcp`, with OAuth 2.1 authorization (PKCE and dynamic client registration) issued by core. Assistants would then connect with only the URL and sign in through the browser, while reusing these toolsets.
