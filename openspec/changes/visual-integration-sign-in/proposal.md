## Why

Connecting an integration still takes a shell on a server. A subscription account (Claude, Codex, Cursor) is signed in with `genwork-runner account login`, an interactive command that needs a TTY on the runner's machine. Bots push and open pull requests with a `GH_TOKEN` that someone pastes into the runner's environment. An MCP server that uses OAuth needs a token copied by hand into a header. Each of these should be one click in the web app: the admin clicks, authorizes on the provider's own page, and Genwork keeps the result.

### Constraint: no Genwork-owned Claude.ai OAuth

Anthropic's terms forbid third-party apps from offering Claude.ai login or from collecting, storing, or relaying Claude.ai credentials or session tokens: "sign-in to a Claude account must complete through Anthropic's own flow" (code.claude.com/docs/en/legal-and-compliance, *Authentication and credential use*). Anthropic also has no OAuth client registration for third parties, so Genwork has no client id, secret, or redirect URL of its own. The terms allow an end user to sign in to the **unmodified Claude Code binary** with their own subscription, including where a platform hosts Claude Code. This change therefore starts the provider CLI's own sign-in on the runner and drives it from the web app. The OAuth exchange and the resulting credentials stay inside the CLI, on the runner, as they do today.

## What Changes

1. **Subscription sign-in from the web app (genagents, runner, web).** A subscription account gets a "Log in with <provider>" button. Genagents records a sign-in request. The runner that holds the account picks it up and starts the CLI's own sign-in (`claude auth login`, `codex login --device-auth`, `cursor-agent login`) in a sandbox container. The container gets a pseudo-terminal through the engine API and mounts only that account's profile. The runner relays the authorization URL and any device code to the web app. For Claude, the admin pastes back the code that Anthropic's page shows, and the runner types it into the CLI. The CLI exchanges the code and writes its credentials into the account's profile on the runner. Genagents never sees a provider token. Cancelled, expired, and failed sign-ins are visible on the account. `genwork-runner account login` stays as a fallback.
2. **GitHub credentials per run from the GitHub App (gentasks, genagents, runner).** Gentasks already holds a GitHub App created through the manifest flow. It now issues short-lived installation tokens scoped to the repositories bots work in. Genagents asks for one per run with the bot's own Genwork token. The runner uses it on the host to clone, push, and open pull requests, and renews it before it expires. A runner's own `GH_TOKEN` stays as a fallback. A runner without one can now take GitHub runs.
3. **OAuth for bot MCP servers (genagents, web).** An HTTP MCP server gets a "Connect" button. Genagents runs the MCP authorization flow as an OAuth client: protected resource metadata, authorization server metadata, dynamic client registration or a client id the admin enters, PKCE, and the `resource` parameter. It stores the tokens encrypted, refreshes them, and adds the `Authorization` header when a run claims the server.
4. **Version 2.2.0.**

Components touched: **genagents** (sign-in sessions, GitHub token relay, MCP OAuth), **runner** (sign-in sessions, per-run GitHub tokens), **gentasks** (repository tokens), **web** (accounts and MCP server screens). Core, mcp, cli, and native clients are unchanged. The span is justified because each flow crosses a public contract: the runner API (genagents ↔ runner) and gentasks' versioned API (genagents → gentasks, rule 54).

Delivery note: the developer asked for this change to ship as one pull request, an explicit exception to rules 42–46.

## Capabilities

### New Capabilities

- `genagents-subscription-sign-in`: web-driven sign-in of subscription accounts through the provider CLI on the runner.
- `genagents-mcp-oauth`: OAuth connections for HTTP MCP servers used by bots.
- `genagents-run-github-credentials`: per-run GitHub credentials relayed from gentasks to runners.
- `gentasks-github-repository-tokens`: short-lived installation tokens from the gentasks GitHub App.

### `module.yaml` changes

- gentasks: new permission `gentasks.repository.write` [admin, member]. Only bot members get a token; a person holding the permission is refused. Core has no screen to edit role permissions yet, so a member-role bot works out of the box.
- genagents: none.
