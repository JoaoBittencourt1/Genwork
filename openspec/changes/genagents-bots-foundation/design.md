## Context

See proposal.md for motivation and `genagents-orchestrator` for the full vision this change starts. Relevant current state:

- Core owns members (`users`, one role each), sessions with rotating refresh tokens (rule 69), and an OAuth 2.1 server used by the MCP sign-in (authorization code + PKCE only). `users.password_hash` is `NOT NULL`.
- gentasks assigns tasks to `assignee_user_token`, already exposes the caller's assigned tasks, and owns a GitHub App (webhooks only, no API calls) that links pull requests citing a task key. Boards have no repository field.
- The installer already provisions `<PREFIX>_ENCRYPTION_KEY` for every module, and gentasks encrypts secrets with AES-256-GCM keyed by it with "column + row token" as associated data. genagents reuses that pattern.
- The MCP server exposes core and gentasks toolsets, filtered by capabilities and the caller's permissions, and forwards the caller's token per request. genmonitor has no tools yet.
- The web app polls every 10 s by default (`web-live-refresh`); there is no server push.
- The constitution leaves the event bus open; the developer chose direct, versioned HTTP calls for this delivery.

## Goals / Non-Goals

**Goals:** a working end-to-end loop (link an account → create a bot → assign a task → bot opens a PR while chatting → bot review on GitHub), plus the admin chat, with identity and execution foundations the orchestrator can extend without rework.

**Non-Goals:** orchestrator features (see proposal), visual polish, server push, provider-API execution without a runner, and native clients.

## Decisions

### 1. A new module, `genagents`, plus generic core extension points
Everything about AI accounts, bots, skills, runners, runs, and conversations is a module (rule 49). Core only gains two generic concepts any module can use: **bot members** and **delegated sessions** (rule 52); neither mentions genagents. Alternative: bots as module-only records with a separate `assignee_bot_token` in gentasks. Rejected by the developer: bots would have no permissions of their own in the MCP and every module would need its own bot assignee concept (rule 7).

### 2. Bot members authenticate with OAuth `client_credentials`
`users` gains `kind text NOT NULL DEFAULT 'human' CHECK (kind IN ('human','bot'))`, `password_hash` becomes nullable with `CHECK ((kind = 'human') = (password_hash IS NOT NULL))`, and `email` becomes nullable for bots under the same rule (the e-mail uniqueness index becomes partial on humans). A new `bot_credentials` table holds `user_id`, `secret_hash` (SHA-256 of a 256-bit random secret, rule 67), `created_at`, and `rotated_at`. The existing token endpoint accepts `grant_type=client_credentials` with HTTP Basic (`bot token : secret`), compares the hash in constant time, and issues the standard access token with no refresh token. Login treats bots as unknown accounts (rule 68).

Alternatives: long-lived personal access tokens (rejected: a bearer that never expires is exactly what rule 69 avoids); a module minting tokens for any bot (rejected: core would trust modules to impersonate, a bigger blast radius).

### 3. genagents keeps bot secrets and API keys encrypted, not hashed
genagents must *use* the bot secret (to obtain tokens) and the provider API keys (to hand them to a run), so a hash is impossible. They are stored as AES-256-GCM ciphertext with `GENAGENTS_ENCRYPTION_KEY`, as gentasks does for the GitHub App key, and never logged or returned. This satisfies rule 67's "never in clear" and follows the precedent gentasks set. Subscription credentials never reach genagents at all (decision 5).

### 4. Delegated sessions for the admin chat
`sessions.client_type` gains `delegated`, plus nullable `delegated_module_id` and `purpose`. `POST /api/core/v1/delegated-sessions` (any signed-in user, for themselves; body: module name, purpose) creates the session and returns its refresh token once. Rotation and reuse detection are the existing ones. genagents stores the current refresh token encrypted and rotates it on each use; core recomputes permissions on every refresh, so a role change applies within 15 minutes. Alternative: run the admin chat as a bot with the `admin` role. Rejected: that bot would be an admin everywhere (task assignment, MCP), and actions would not be attributed to the admin.

### 5. Execution happens on runners, never in the module container
Subscription plans (Claude Pro/Max, ChatGPT plans, Cursor) can only be used through each provider's official CLI and its own sign-in, and rule 84 forbids modules from touching Docker. So work runs on a **runner**, a new Go component in `runner/` (`genwork-runner`) that the admin installs on a machine they control, with Docker or Podman. Each run executes in a disposable sandbox container (decision 7a). Same shape as Paperclip's local adapters, with isolation added. Rule 84 is about the Genwork stack's containers; the runner is not part of the stack and publishes no port, so its use of the local container engine on its own machine is outside that rule. Phase 22 adds a sentence to rule 84 saying so.

- **Transport.** HTTPS through the gateway under `/api/genagents/v1/runner/...`. The runner authenticates with `Authorization: Bearer <runner secret>`, a 256-bit secret whose SHA-256 is stored (rule 67). No inbound port on the runner.
- **Claiming.** `POST /runner/claims` long-polls for up to 25 s. The server picks the oldest eligible queued run with `SELECT … FOR UPDATE SKIP LOCKED`, sets `runner_id`, `claimed_at`, and state `running`, and returns the run spec. Eligibility: subscription account bound to this runner, or API-key account whose provider CLI the runner reported, and GitHub credentials present for task and review runs.
- **Run spec snapshot.** On claim the full spec (prompt, instructions, model, parameters, skill contents, MCP servers with decrypted secrets, API key) is built and returned once; the copy stored in `runs.spec` holds only references and the skill versions, never secrets.
- **Events.** `POST /runner/runs/{token}/events` with a batch of `{sequence, type, payload}`, where `UNIQUE (run_id, sequence)` makes retries idempotent. Types: `status`, `assistant_message`, `tool_use`, `result`, `error`.
- **Human messages.** `GET /runner/runs/{token}/messages?after=` returns undelivered messages; the runner resumes the CLI session with them at the next turn boundary and acknowledges delivery.
- **Genwork tokens.** `POST /runner/runs/{token}/genwork-token` returns a fresh access token for the run's identity (bot member via client credentials, or admin via the delegated session), only while the run is active.
- **Liveness.** `POST /runner/heartbeat` every 15 s carries CLI versions, GitHub credential presence, free slots, and active run tokens. A sweeper fails runs whose runner has been silent for 5 minutes (`runner_lost`), and runs past their timeout (`timeout`).

### 6. Provider adapters in the runner
One `ProviderAdapter` interface (rule 9) per provider: `DetectCLI`, `SignIn(profileDir)`, `PrepareWorkspace(spec)`, `Start(spec) → event stream`, `Resume(sessionID, message)`, `Stop`. Each adapter maps CLI output (stream JSON where available) to run events. The CLI flags below are what each CLI offers today; the adapter is the only place that knows them.

| Provider | CLI | Per-account profile | API key | Skills | MCP |
|---|---|---|---|---|---|
| anthropic | Claude Code (`claude -p --output-format stream-json`, `--resume`) | `CLAUDE_CONFIG_DIR` | `ANTHROPIC_API_KEY` | `.claude/skills/<name>/` in workspace | `--mcp-config` file |
| openai | Codex (`codex exec --json`, resume) | `CODEX_HOME` | `OPENAI_API_KEY` | prompt fallback + files in workspace | per-run `config.toml` in a copied `CODEX_HOME` |
| cursor | Cursor agent CLI (`-p --output-format stream-json`, resume) | its config dir via `HOME` override | `CURSOR_API_KEY` | prompt fallback + `.cursor/rules` | `.cursor/mcp.json` in workspace |
| xai | Grok CLI (headless prompt mode) | n/a | `GROK_API_KEY` | prompt fallback | its MCP config file |

xAI has no official subscription sign-in for a CLI, so `xai` supports `api_key` only; the account form hides `subscription` for it. Runs execute non-interactively with the CLI's "allow all tools" mode, because nobody can approve prompts on a headless runner. The sandbox (decision 7a) is the boundary, not the CLI's own permission prompts. Adapters build the CLI command line; the sandbox launcher wraps it in the container, so an adapter never talks to the container engine.

### 7. Workspaces and Git
Each run gets `<runner data dir>/runs/<run token>/` with a shallow clone of the board repository over HTTPS, made **on the host** with the runner's own GitHub token (from the runner's config file or `GH_TOKEN`). The token reaches Git through a one-off credential helper and is never written into the repo config. The clone is checked out on `bot/<task-key-lowercase>-<short run token>` before the sandbox starts. Inside the sandbox the bot commits locally with the bot's name as author and has no Git remote credentials. When the run ends with a result (PR title and body), the runner pushes the branch from the host and opens the pull request with the GitHub REST API. No commits means no push and no PR. Review runs check out the PR head (host), the model reads it inside the sandbox, and the runner posts the review through the REST API (`event: COMMENT`) from the structured result the model returns. The workspace is deleted when the run ends. Alternative: use gentasks' GitHub App for clone and posting. Rejected for now: the App has no installation-token flow and only webhook permissions, and adding one would mean every installation re-approving App permissions. It is listed in the orchestrator roadmap.

### 7a. Sandbox per run
The `SandboxLauncher` interface (rule 9) has two implementations: `ContainerSandbox` (default) and `DirectorySandbox` (explicit `--directory-mode` flag, development only, flagged on the Runners page and used only when no sandboxed runner is eligible). `ContainerSandbox` talks to the Docker Engine API, which Podman also serves, through the local socket from the runner process, never from inside a sandbox. Container settings:

- Image: `genwork-sandbox:<VERSION>` from the release registry by default, or the runner's configured custom image. Pulled by digest; the digest is reported in the heartbeat.
- `--rm`, label `genwork.run=<run token>`, non-root UID, `--cap-drop=ALL`, `--security-opt=no-new-privileges`, `--read-only` with tmpfs `/tmp` and `$HOME`, plus `--pids-limit`, `--memory`, and `--cpus` from runner config.
- Mounts: the run workspace at `/workspace` (rw); for subscription accounts, only that account's profile directory at the CLI's config path (rw, because CLIs refresh their own tokens). Nothing else from the host.
- Network: the engine's default bridge (outbound only). Bots need provider APIs, the Genwork MCP, and package registries. An egress allowlist is a later improvement (orchestrator roadmap).
- Environment: the API key (API-key accounts), the Genwork MCP URL, and its short-lived token. The token is refreshed by the runner, which rewrites the MCP config file in the workspace, because the sandbox cannot call the runner.
- Lifecycle: start → stream CLI output from the container's stdout → stop on result, cancel, or timeout (`docker stop`, then kill after 10 s) → remove. On start, the runner removes containers labelled with runs it does not hold. An OOM-killed container fails the run with `sandbox_limit`.

Resuming a session with a human message starts a new container on the same workspace and profile, running the CLI's resume command. The workspace stays until the run ends, so the session state the CLI keeps under the workspace or profile survives between turns.

The sandbox image is built from `runner/sandbox/Dockerfile` by `make build` and the release workflow. It installs Git, Claude Code, Codex, the Cursor agent CLI, the Grok CLI, Go, Node.js, and Python, with CLI versions pinned in that Dockerfile, their single owner (rule 7).

Alternatives: a cloud sandbox service (rejected by the developer: paid, external dependency, harder subscription sign-in); directory isolation only (kept just as the development mode).

### 8. Prompts are built by genagents, not the runner
genagents composes the run prompt: a fixed template per run kind (task, review, chat) plus bot instructions plus task context read from gentasks. The runner stays a generic executor, and changing a template never requires redeploying runners. The review template asks for structured output (JSON findings with path, line, severity, and message) that the runner validates before posting.

### 9. gentasks ↔ genagents over versioned HTTP, with reconciliation
- **gentasks → genagents:** `POST /api/genagents/v1/task-assignments` (hint, caller's token forwarded) after an assignee change involving a bot member, and the `bot-reviews` proxy. gentasks learns whether genagents is enabled from core capabilities (rule 58) and reaches it through the gateway's internal URL (`GATEWAY_INTERNAL_URL`, already used by mcp).
- **genagents → gentasks:** the task context read (new `GET /api/gentasks/v1/tasks/{token}/context`) and the existing assigned-tasks list, called with the bot member's token for reconciliation (every 60 s) or the requester's token for reviews.

Reconciliation removes the need for an outbox in gentasks: it cannot keep the user's token to retry later. When the platform picks an event bus, both directions become events (orchestrator roadmap).

### 10. Data model (schema `genagents`)
Every table has `id` and `token` (rule 14), timestamps, and `deleted_at` where removable (rule 22).

- `ai_accounts`: name, provider, mode, `runner_id` (subscription), `api_key_ciphertext`, `api_key_last4`, status, `last_error`
- `ai_account_models`: `account_id`, model id, display name (custom models)
- `runners`: name, `secret_hash`, `registration_token_hash`, `registration_expires_at`, `last_heartbeat_at`, `reported_tools jsonb`, `has_github_credentials`, `max_concurrency`, `revoked_at`
- `skills`, `skill_files` (`skill_id`, path, content), `skills.version` incremented on edit
- `mcp_servers`: name, transport, `config jsonb` (non-secret), `secrets_ciphertext`
- `bots`: name, kind, `account_id`, model, effort, `max_turns`, `timeout_minutes`, instructions, `member_token`, `member_secret_ciphertext`, `disabled_at`; plus `bot_skills` and `bot_mcp_servers` join tables
- `conversations`: kind (`task`, `admin_chat`), `task_token` (unique among active task conversations), `owner_user_token`, title, `delegated_refresh_ciphertext`
- `runs`: `conversation_id`, `bot_id`, kind (`task`, `review`, `chat`), state, `failure_reason`, `requester_user_token`, `runner_id`, `cli_session_id`, `spec jsonb`, `claimed_at`, `ended_at`; partial unique index on (`conversation_id`, kind) for active states (one active task run and one active review per task)
- `conversation_entries`: `conversation_id`, `run_id`, sequence (per conversation), type (`human_message`, `status`, `assistant_message`, `tool_use`, `result`, `error`), `author_token`, `payload jsonb`, `delivered_at` (human messages)

Secrets never outlive their record: soft-deleting an account, MCP server, bot, or conversation also sets its ciphertext columns to `NULL` in the same statement, and `runners.registration_token_hash` is cleared on exchange. No physical delete is needed (rule 22).

### 11. Authorization
| Endpoint group | Rule |
|---|---|
| accounts, catalog | `genagents.account.manage` |
| bots read / write | `genagents.bot.read` / `genagents.bot.manage` (bot creation also needs the caller's `core.member.manage`, checked by core) |
| skills, MCP servers, runners admin | their `manage` permission |
| task conversation read / message / stop | `genagents.run.read` / `genagents.run.write`, plus `gentasks.board.read` on the task (checked by reading the task as the caller) |
| task-assignment hint, review request | caller token forwarded; the assignment is re-verified (decision 9); review needs `genagents.run.write` |
| admin chat | `genagents.adminchat.use` and conversation owner = caller |
| `/runner/...` | runner secret; each run endpoint checks `runs.runner_id` = caller |

Ownership checks (conversation owner, run belongs to runner) live in one `RunAccessPolicy` (rule 18).

### 12. MCP toolsets
`mcp/internal/genmonitor` (host snapshot read) and `mcp/internal/genagents` (bots list, runs list and read, conversation read and post, review request) follow the existing toolset pattern and permission filter. Bot-member tokens already verify like user tokens (same issuer and keys); `/me` returns `kind`, which needs no MCP change.

### 13. Web
`web/src/app/genagents/` with the rule 72 split. Pages are shadcn tables and dialogs with no custom styling. The conversation and admin chat poll with `refetchInterval` from a pure `conversationRefreshInterval({ activeRun })` (2 s active, 10 s idle) in `domain/`. gentasks' task dialog renders the conversation panel from its own `web/src/app/gentasks/` code calling the genagents API through a gentasks datasource (rule 59 forbids importing genagents UI). The shared conversation types live in `web/src/shared/domain/` because both modules read them (rule 7).

### 14. Runner distribution
`runner/` gets its own `go.mod`, `cmd/genwork-runner`, and `internal/` (rule 82). It is built for linux/darwin amd64/arm64 by `make build` and the release workflow, versioned by `VERSION` (rule 50), and documented in `runner/README.md`. The sandbox image (decision 7a) is published with the same version. The constitution's component table gains `runner/`.

## Risks / Trade-offs

- [Provider terms may limit automated or shared use of consumer subscriptions] → Documented on the Accounts page and in the runner README; API-key mode is always available; the admin decides which accounts to link.
- [Bots run with tool approval disabled] → Each run is confined to a disposable, non-root, capability-free container that sees only its workspace and its own account profile. GitHub credentials, the runner secret, and the engine socket never enter it. The bot member's role bounds what it can do in Genwork.
- [The sandbox has open outbound network, so a prompt-injected bot could exfiltrate the workspace or the API key it holds] → The workspace only holds the repository the bot already works on. Use API keys dedicated to bots with provider-side spend limits. An egress allowlist is on the roadmap.
- [Container escape through an engine or kernel vulnerability] → Keep the engine updated. The README recommends rootless Podman or Docker on a dedicated machine.
- [Sandbox image size and pull time] → One image per release, pulled once and reused; custom images are opt-in.
- [Directory mode is unsafe] → Only with an explicit flag, flagged in the UI, and used only as a last resort.
- [Prompt injection from task descriptions or PR diffs] → Review bots only comment (never approve or merge); dev bots never merge; bot roles default to `member`; tool activity is visible in conversations.
- [CLI output formats change between versions] → Adapters are the only parsers; the heartbeat reports CLI versions; an unparseable stream fails the run with `adapter_error` and the raw tail in the conversation.
- [Polling load from conversations] → Incremental reads by sequence; 2 s only while a run is active; hidden tabs stop polling.
- [Bot secret stored reversibly in genagents] → Encrypted with a per-module key, rotatable from the bot page (rotates in core and genagents together).
- [Several moving parts for one delivery] → The phase plan delivers core and gentasks first, then the module API, then runner, then UI, each independently testable with stubs (rules 44, 62).

## Migration Plan

Core migration adds columns with defaults (existing members become `human`); gentasks migration adds a nullable column. genagents is a new module the installer offers; enabling it later needs no change elsewhere (rule 56). Runners are optional until someone assigns a task. Rollback: disable genagents (gentasks hides bot features); the core columns are additive and stay.

## Open Questions

- Default concurrency per runner (2) and reconciliation period (60 s) may be tuned after manual verification without spec changes.
