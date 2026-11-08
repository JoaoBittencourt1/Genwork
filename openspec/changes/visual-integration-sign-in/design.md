## Context

Subscription sign-in, GitHub credentials for runs, and MCP server credentials all need a person at a server today. This design keeps every credential with its rightful owner. Provider CLI credentials stay with the CLI on the runner. GitHub App credentials stay in gentasks. MCP OAuth tokens stay in genagents, encrypted. The web app only starts and watches each flow.

## Decisions

### 1. Subscription sign-in is the CLI's own flow, relayed

Anthropic forbids a third party from running Claude.ai OAuth itself or from storing Claude.ai tokens (see proposal). Codex and Cursor sign-ins are also only supported through their CLIs. So the runner starts the CLI's sign-in in a sandbox and relays only what a person has to see or type:

| Provider | Command | What the person does |
| --- | --- | --- |
| anthropic | `claude auth login` | Opens Anthropic's URL, authorizes, and pastes the code Anthropic shows into Genwork |
| openai | `codex login --device-auth` | Opens OpenAI's device page and enters the code Genwork shows |
| cursor | `cursor-agent login` (`NO_OPEN_BROWSER=1`) | Opens Cursor's URL and authorizes; the CLI polls |

Each runner adapter owns its command, its environment, whether it expects a pasted code, and the pattern of its device code (rule 7). The provider catalog in genagents says which providers offer a subscription sign-in. It already does, through `Modes`.

**State machine** (`account_sign_ins.state`):

```
requested ──runner starts CLI──▶ awaiting_user ──person pastes code──▶ code_submitted ──runner types it──▶ verifying
    │                               │                                                                     │
    └──────── cancelled / expired / failed ◀──────────────────────────────────────────────────────────────┴──▶ succeeded
```

- At most one active sign-in per account (partial unique index). Starting a new one cancels the active one.
- A sign-in expires 10 minutes after it is requested. Genagents marks it expired when it is read. The runner kills the container at the same deadline.
- On success the account becomes `ready` and its `last_error` is cleared. On failure or expiry it becomes `error` with a message. Cancelling leaves the account's status as it was, and the cancelled attempt shows on the account.
- A run that fails because the provider refuses the credentials already moves the account to `error` (a revoked session). The button then offers a new sign-in.

**Where the pasted code lives.** It is encrypted with the module's `SecretCipher` (associated data `account_sign_ins.code|<token>`). The runner's poll is the only place it is returned. The column is cleared as soon as the runner reports `verifying`. No API response to a person, log line, or MCP tool carries it.

**Runner side.** A new sign-in loop polls `GET /runner/sign-ins` every 3 seconds. The loop is cheap, and a heartbeat every 15 seconds would make the button feel broken. For each new request it creates a container with `Tty`, `OpenStdin`, and `StdinOnce`, attaches with an HTTP upgrade before starting it, and resizes the terminal to 500 columns so the URL never wraps. It strips terminal escapes from the output, takes the last `https://` URL and the device code, and reports them. When the poll returns a code, it types the code and a carriage return. The exit code decides success. The container has the same isolation as a run and mounts only the account's profile and an empty scratch workspace. Its label is `genwork.signin`, so leftover cleanup never reports it as a lost run.

**Why not `claude setup-token` + `CLAUDE_CODE_OAUTH_TOKEN`?** That would make Genwork collect and store a Claude.ai token, which the terms forbid. The relayed `auth login` keeps the token in the CLI's own profile.

### 2. GitHub credentials per run come from the GitHub App

- gentasks `POST /api/gentasks/v1/github/repository-tokens` `{repositories: ["owner/name", …]}` (1–10) returns `{token, expiresAt}`. It requires `gentasks.repository.write` (default roles admin and member) **and a bot member caller**, checked through core's user directory (`kind = bot`), so no person — not even an admin — turns the App's write access into a way around their own GitHub rights. It only accepts repositories configured on a board or linked to a task's pull request, so the endpoint cannot reach arbitrary repositories of the installation. It signs an App JWT (RS256, 9 minutes) with the stored private key, finds the installation of the first repository, checks every repository belongs to it, and requests a token limited to those repositories with `contents: write`, `pull_requests: write`, and `metadata: read`. The token is returned with `Cache-Control: no-store` and never stored.
- The App manifest asks for `contents: write` and `pull_requests: write`. An App created before this change keeps `pull_requests: read` until its owner accepts the new permissions on GitHub. GitHub then answers 422 and the error says so.
- genagents `POST /runner/runs/{token}/github-token` issues the bot's Genwork token, calls gentasks with it, and answers `{token, expiresAt}`, or 409 `github_token_unavailable` with a `reason`.
- The runner asks for it at set-up and on every token refresh. It uses the token for git on the host and for the REST calls, and falls back to its own `GH_TOKEN`. If neither exists, the run fails with `github_credentials_missing`.
- Eligibility no longer requires `HasGithubCredentials`: any runner can now get credentials.

### 3. MCP OAuth follows the MCP authorization spec

- `POST /mcp-servers/{token}/oauth/authorizations` (`genagents.mcpserver.manage`) runs discovery:
  1. `GET <server>` without a token, reading `resource_metadata` from `WWW-Authenticate`.
  2. Otherwise `/.well-known/oauth-protected-resource` at the path and at the root.
  3. Then `/.well-known/oauth-authorization-server`, falling back to `openid-configuration`.
  4. If the admin entered no client id, it registers dynamically (RFC 7591) with redirect `GENWORK_PUBLIC_URL/api/genagents/v1/mcp-servers/oauth/callback`.

  It creates an authorization with a random state (stored hashed) and a PKCE verifier (encrypted), both expiring after 10 minutes, and returns the authorization URL with `resource` (RFC 8707).
- `GET /mcp-servers/oauth/callback` is public, because it is a browser redirect. It is authorized by the single-use state. It exchanges the code, stores the tokens encrypted, and redirects to `/ai/mcp-servers?mcpOAuth=connected|failed`.
- Claim resolution refreshes an access token that expires within 5 minutes and adds `Authorization: Bearer …` to the server's headers for the run. A refresh failure marks the connection `error`, and the run starts without the header. Long runs keep the token they started with (a follow-up may renew it with the Genwork token).
- Responses show only `{status, connectedAt, expiresAt, lastError, clientId}`, never a token or secret.
- Disconnecting deletes the connection row physically, a documented exception to rule 22: the row holds only credentials (rule 67), and keeping it soft-deleted would keep tokens at rest for nothing. Sign-in attempts and authorizations are never deleted; they end in a final state.
- Discovery and token calls refuse non-HTTPS endpoints, except loopback, which tests use.

## Risks / Trade-offs

- The CLIs' terminal output is not a stable contract. URL and code extraction is per adapter and tolerant. When nothing is found, the sign-in fails with the redacted output tail, and the CLI command remains as a fallback.
- A sandbox image could print a phishing URL. The UI shows the URL's host next to the link, and only `https` URLs are accepted.
- Advertised subscription limits assume ordinary individual use. The accounts screen keeps its terms note.
- The GitHub App needs new permissions. Existing installations must accept them on GitHub before platform tokens work. Until then `GH_TOKEN` keeps working.
