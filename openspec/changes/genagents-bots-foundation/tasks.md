### Phased delivery

Dependency graph, by layer (rules 43–45). Go phases follow rule 47's order (test → migration → model → repository → authorization → request type → service → handler → response type). Each phase is one branch and one PR, capped at ~600 productive lines and 10 new files (rule 46); a phase that grows beyond that is split before implementation.

> Delivered as one pull request, by the developer's decision, instead of one PR per phase (rules 42–48). The phase
> structure below is kept as the map of the work. Phase 23.2 (manual verification) and 23.3 (archive) stay open until
> the developer signs off.

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | core: bot member migration, model, repository | — | 4, 5 |
| 2 | core: bot member API, `client_credentials` grant, `kind` in member/directory/me | 1 | 3 |
| 3 | core: delegated sessions | 1 | 2 |
| 4 | gentasks: bot assignees, board `repository`, task context read | — (bot kind stubbed from core fixtures) | 1, 5 |
| 5 | genagents: module skeleton, encryption, accounts and model catalog foundation (migration → repository) | — | 1, 4 |
| 6 | genagents: accounts API | 5 | 7 |
| 7 | genagents: skills and MCP servers (migration → handler) | 5 | 6 |
| 8 | genagents: bots, with the core bot member client | 2, 6, 7 | 9 |
| 9 | genagents: runner API (registration, heartbeat, claims, events, messages, Genwork tokens, sweeper) | 5 | 8 |
| 10 | genagents: task runs, conversations, reconciliation, reviews | 4, 8, 9 | 11 |
| 11 | genagents: admin chat | 3, 8, 9 | 10 |
| 12 | runner: foundation (register, config, heartbeat, profiles, claim loop, host workspace and clone, event posting) with stub adapter and stub sandbox | 9 | 14 |
| 13 | runner: sandbox (container launcher over the Engine API, directory mode, sandbox image) | 12 | 14 |
| 14 | gentasks: assignment hint client and bot review proxy | 4, 10 | 12, 13 |
| 15 | runner: Claude Code and Codex adapters, skills and MCP config | 13 | 16, 17 |
| 16 | runner: Cursor and Grok adapters, host-side push, PR, and review posting | 13 | 15, 17 |
| 17 | mcp: genmonitor and genagents toolsets | 10, 11 | 15, 16, 18 |
| 18 | web: genagents and gentasks logic (types, datasource, domain, MSW fixtures, tests) | 10, 11, 14 (contracts in specs) | 17 |
| 19 | web: genagents management pages | 18 | 20, 21 |
| 20 | web: admin chat page | 18 | 19, 21 |
| 21 | web: gentasks assignee picker, board repository field, conversation panel, "Bot review" | 18 | 19, 20 |
| 22 | release: runner binaries and sandbox image, installer listing, constitution updates, docs | 13 | 15–21 |
| 23 | full gates and manual end-to-end verification | 1–22 | — |

## Phase 1 — core bot member foundation (branch: feat/core-bot-members-foundation)

- [x] 1.1 Write repository tests in `core/tests/users/`: an existing member migrates to kind `human`; a bot row requires no e-mail or password and a human row still requires both; e-mail uniqueness ignores bots; creating, finding by token, and rotating a bot credential stores only the hash; a deactivated bot is not returned as active
- [x] 1.2 Write `core/migrations/00012_bot_members.sql` (design §2) with a working `Down`
- [x] 1.3 Add `MemberKind` (typed, rule 15) to the user model and a `BotCredential` model
- [x] 1.4 Implement `BotCredentialRepository` and extend the user repository for kind
- [x] 1.5 Run `make lint` and core tests
- [x] 1.6 Open PR

## Phase 2 — core bot member API (branch: feat/core-bot-members-api)

- [x] 2.1 Write tests in `core/tests/app/` and `core/tests/oauth/`: create, rename, change role, deactivate, remove, and rotate a bot need `core.member.manage` (403 otherwise); create and rotate return the secret once; `client_credentials` issues a ≤15 min access token without a refresh token; wrong secret, deactivated bot, and unknown client return the same error; rate limit per bot; password login for a bot name returns the generic error; member list, directory, and `me` include `kind`; `PATCH` single-field test (rule 17)
- [x] 2.2 Add the authorization rules, request types, `BotMemberService`, `BotMemberHandler`, and response types under `/api/core/v1/bots`
- [x] 2.3 Add the `client_credentials` grant to the OAuth token endpoint
- [x] 2.4 Add `kind` to member, directory, and `me` responses (additive, rule 61)
- [x] 2.5 Run `make lint` and core tests
- [x] 2.6 Open PR

## Phase 3 — core delegated sessions (branch: feat/core-delegated-sessions)

- [x] 3.1 Write tests: grant needs a signed-in user and an enabled module (400 otherwise); refresh token returned once; rotation and reuse revocation behave like other sessions; listed in the user's sessions with module and purpose; ended by the user, by member deactivation, and by revoke; permissions recomputed on refresh after a role change; idle and absolute limits
- [x] 3.2 Write `core/migrations/00013_delegated_sessions.sql` with a working `Down`
- [x] 3.3 Extend the session model and repository; add the request type, service method, handler `POST /api/core/v1/delegated-sessions`, and response type
- [x] 3.4 Run `make lint` and core tests
- [x] 3.5 Open PR

## Phase 4 — gentasks bot assignees and task context (branch: feat/gentasks-bot-assignees)

- [x] 4.1 Add recorded core directory fixtures with a bot member
- [x] 4.2 Write tests: a bot member is accepted as assignee; board view and task reads carry `assigneeKind`; board GitHub settings `PATCH repository` validates `owner/name` and leaves other fields unchanged (rule 17); `GET /tasks/{token}/context` needs `gentasks.board.read` and returns key, title, description, labels, board, linked pull requests, and repository
- [x] 4.3 Write `modules/gentasks/migrations/00006_board_repository.sql`
- [x] 4.4 Implement the repository field, assignee kind, and the context read (repository → service → handler → response)
- [x] 4.5 Run `make lint` and gentasks tests
- [x] 4.6 Open PR

## Phase 5 — genagents skeleton and accounts foundation (branch: feat/genagents-module-foundation)

- [x] 5.1 Write tests: module registers with a stubbed core; startup fails without `GENAGENTS_ENCRYPTION_KEY` or with a short key; encrypt/decrypt round trip and associated-data mismatch failure; account repository create/list/soft delete clears ciphertext; custom model uniqueness per account
- [x] 5.2 Create `modules/genagents/` (`go.mod`, `cmd/genagents`, `internal/command`, `internal/app`, `manifest.go`, `module.yaml` from the proposal, `Dockerfile`, `migrations/embed.go`)
- [x] 5.3 Write `00001_ai_accounts.sql` (accounts, account models, runners table needed by subscription accounts)
- [x] 5.4 Add `Provider`, `AccountMode`, `AccountStatus` types, models, and repositories
- [x] 5.5 Wire the module into `make lint`, `make test`, `make build`, and CI (rule 75)
- [x] 5.6 Run `make lint` and genagents tests
- [x] 5.7 Open PR

## Phase 6 — genagents accounts API (branch: feat/genagents-accounts-api)

- [x] 6.1 Write tests: `genagents.account.manage` required; several accounts per provider; unique names; `api_key` never returned (only `last4`); subscription needs a runner reporting the provider CLI and starts `pending_sign_in`; xAI rejects `subscription`; removing an account in use returns 409 with bot names; model catalog per provider plus custom models; `PATCH` single-field test
- [x] 6.2 Add the provider catalog (single owner of models per provider, rule 7)
- [x] 6.3 Implement authorization, request types, `AiAccountService`, handler, and response types
- [x] 6.4 Run `make lint` and genagents tests
- [x] 6.5 Open PR

## Phase 7 — genagents skills and MCP servers (branch: feat/genagents-skills-mcp-servers)

- [x] 7.1 Write tests: skill name and description rules; path traversal and absolute paths rejected; file count and size limits; `SKILL.md` import from front matter; version increments on edit; MCP server `http` and `stdio` validation; secret headers and env never returned; deleting detaches from bots; permissions required
- [x] 7.2 Write `00002_skills_mcp_servers.sql`
- [x] 7.3 Implement models, repositories, authorization, request types, services, handlers, and response types for both features
- [x] 7.4 Run `make lint` and genagents tests
- [x] 7.5 Open PR

## Phase 8 — genagents bots (branch: feat/genagents-bots)

- [x] 8.1 Add recorded core fixtures for bot member create, rename, role change, deactivate, remove, rotate, and token
- [x] 8.2 Write tests: create creates the bot member and stores its secret encrypted; core refusal leaves no bot; model must belong to the account's catalog or custom models; kind rules; disable deactivates the member and cancels active runs; delete removes the member; rotate secret; reads never expose secrets; `PATCH` single-field test; permissions
- [x] 8.3 Write `00003_bots.sql` (bots, bot skills, bot MCP servers)
- [x] 8.4 Implement `CoreBotMemberClient` (interface at the boundary, rule 9), models, repository, authorization, request types, `BotService`, handler, and response types
- [x] 8.5 Run `make lint` and genagents tests
- [x] 8.6 Open PR

## Phase 9 — genagents runner API (branch: feat/genagents-runner-api)

- [x] 9.1 Write tests: registration token single use and one-hour expiry; secret stored as hash; revoked runner rejected and its runs fail `runner_revoked`; heartbeat updates presence and tools; online/offline threshold; claims are atomic under concurrency and respect eligibility (subscription binding, provider CLI, GitHub credentials); claim returns secrets once and stores a secret-free spec; events idempotent by sequence; message fetch and delivery acknowledgment; Genwork token only for active runs; sweeper fails `runner_lost` and `timeout`
- [x] 9.2 Write `00004_runs.sql` (conversations, runs, conversation entries)
- [x] 9.3 Implement models, repositories, `RunAccessPolicy`, request types, `RunnerService`, `RunClaimService`, the sweeper, handlers, and response types
- [x] 9.4 Run `make lint` and genagents tests
- [x] 9.5 Open PR

## Phase 10 — genagents task runs, conversations, reviews (branch: feat/genagents-task-runs)

- [x] 10.1 Add recorded gentasks fixtures (task context, assigned tasks) used through a stubbed gentasks API (rule 62)
- [x] 10.2 Write tests: assignment hint verified against the task (forged hint starts nothing); one active task run per task; reassignment cancels; reconciliation starts and cancels runs; `no_repository`; prompt composed from the task template, bot instructions, and context; `waiting_for_input` and the 24 h `no_reply` timeout; conversation read and post permissions; incremental reads; stop; review request rules (single default reviewer, 400 when ambiguous, 409 without PRs or when active); review prompt asks for structured findings
- [x] 10.3 Implement `GentasksClient`, `TaskRunService`, `ReconciliationService`, `BotReviewService`, `ConversationService`, handlers, and response types
- [x] 10.4 Run `make lint` and genagents tests
- [x] 10.5 Open PR

## Phase 11 — genagents admin chat (branch: feat/genagents-admin-chat)

- [x] 11.1 Write tests: every endpoint requires `genagents.adminchat.use`; another admin's chat answers 404; starting a chat grants a delegated session through a stubbed core; only `assistant` bots; each message queues a chat run resuming the CLI session; a revoked session makes the next turn fail with the reopen message; delete revokes the session; rename and list
- [x] 11.2 Implement `AdminChatService`, the core delegated session client, handlers, and response types
- [x] 11.3 Run `make lint` and genagents tests
- [x] 11.4 Open PR

## Phase 12 — runner foundation (branch: feat/runner-foundation)

- [x] 12.1 Write tests in `runner/tests/` against an `httptest` genagents stub: register stores the secret with owner-only file permissions; config requires the instance URL (rule 1); heartbeat reports tools, slots, and GitHub credential presence; the claim loop respects concurrency; events are posted in order with retries; the workspace is created per run (clone on the host, branch checked out before execution) and removed with secret files in every end state; timeout stops the sandbox (stub `SandboxLauncher`); messages are delivered to the adapter at turn boundaries
- [x] 12.2 Create `runner/` (`go.mod`, `cmd/genwork-runner`, `internal/command`, `internal/config`, `internal/registration`, `internal/heartbeat`, `internal/claims`, `internal/workspace`) with the `ProviderAdapter` and `SandboxLauncher` interfaces and stubs for tests
- [x] 12.3 Implement `account login` profile directories per account
- [x] 12.4 Wire the runner into `make lint`, `make test`, `make build`, and CI
- [x] 12.5 Run `make lint` and runner tests
- [x] 12.6 Open PR

## Phase 13 — runner sandbox (branch: feat/runner-sandbox)

- [x] 13.1 Record Docker Engine API fixtures (create, start, logs stream, wait with OOM, stop, remove, list by label) and serve them from an `httptest` socket stub (rule 25)
- [x] 13.2 Write tests: container spec has non-root user, all capabilities dropped, no-new-privileges, read-only root with tmpfs, CPU/memory/pids limits from config, only the workspace and the run's own account profile mounted, bridge network, run label; never mounts the engine socket or passes the runner secret or GitHub token; stop then kill after 10 s; OOM maps to `sandbox_limit`; leftover labelled containers removed on start; startup fails without an engine unless `--directory-mode`; heartbeat reports engine, image digest, CLI versions from the image, and directory mode
- [x] 13.3 Implement the `SandboxLauncher` interface, `ContainerSandbox` over the Engine API, and `DirectorySandbox`
- [x] 13.4 Write `runner/sandbox/Dockerfile` with pinned CLI and toolchain versions, and add its build to `make build` (no push yet)
- [x] 13.5 Run `make lint`, runner tests, and `make build`
- [x] 13.6 Open PR

## Phase 14 — gentasks hint client and review proxy (branch: feat/gentasks-genagents-integration)

- [x] 14.1 Write tests with a stubbed genagents API and core capabilities: assignee change to or from a bot sends the hint after commit; none when genagents is disabled; failure does not fail the assignment; `POST /tasks/{token}/bot-reviews` needs `gentasks.task.write`, forwards body and caller token, relays 400 and 409, and answers 404 when genagents is disabled
- [x] 14.2 Implement `GenagentsClient`, the hint after assignee changes, and the review proxy handler
- [x] 14.3 Run `make lint` and gentasks tests
- [x] 14.4 Open PR

## Phase 15 — runner Claude Code and Codex adapters (branch: feat/runner-claude-codex-adapters)

- [x] 15.1 Record CLI stream fixtures (Claude Code stream JSON, Codex JSON events) for success, tool use, waiting for input, auth failure, and garbage output
- [x] 15.2 Write tests: command line and environment per mode (profile dir vs API key), skills placed where each CLI reads them, MCP config written with the Genwork MCP and the bot's servers, event mapping, resume with a message, `adapter_error` on unparseable output, auth failure reported as account error
- [x] 15.3 Implement both adapters
- [x] 15.4 Run `make lint` and runner tests
- [x] 15.5 Open PR

## Phase 16 — runner Cursor and Grok adapters, GitHub posting (branch: feat/runner-cursor-grok-github)

- [x] 16.1 Record CLI fixtures for Cursor and Grok and GitHub REST fixtures (create pull request, create review with comments, failure)
- [x] 16.2 Write tests: adapters as in 14.2; branch naming; push and pull request from the host only after the run returns a result with commits; no push or PR without commits; pull request title cites the task key; review JSON validated before posting; `github_post_failed` posts the full review text to the conversation; the GitHub token never appears in events or logs
- [x] 16.3 Implement both adapters and the GitHub client
- [x] 16.4 Run `make lint` and runner tests
- [x] 16.5 Open PR

## Phase 17 — MCP toolsets (branch: feat/mcp-genmonitor-genagents-tools)

- [x] 17.1 Write tests in `mcp/tests/`: genmonitor and genagents tools appear only when the module is enabled and the caller holds each permission; tool calls forward the token; bot-member tokens work like user tokens
- [x] 17.2 Implement `mcp/internal/genmonitor` and `mcp/internal/genagents` toolsets
- [x] 17.3 Run `make lint` and mcp tests
- [x] 17.4 Open PR

## Phase 18 — web logic (branch: feat/web-genagents-logic)

- [x] 18.1 Add MSW fixtures and handlers in `web/src/test/` for every genagents endpoint and the new gentasks fields and endpoints
- [x] 18.2 Write tests: datasource request shapes and error mapping; model filtering by account; provider mode rules (xAI has no subscription); page and action visibility from permissions and capabilities; `conversationRefreshInterval`; conversation entry ordering and merge of incremental reads; "Bot review" enabled only with open or draft PRs; reviewer picker rules; assignee grouping of bots
- [x] 18.3 Implement `web/src/app/genagents/{datasource,domain}`, the shared conversation types in `web/src/shared/domain/`, and the gentasks datasource and domain additions
- [x] 18.4 Run `make lint` and web tests
- [x] 18.5 Open PR

## Phase 19 — web management pages (branch: feat/web-genagents-pages)

- [x] 19.1 Add English and pt-BR copy for Accounts, Bots, Skills, MCP servers, and Runners
- [x] 19.2 Implement the five pages as shadcn tables with form dialogs, the AI navigation section, and routes (adding missing shadcn components with `npx shadcn@latest add`, rule 73)
- [x] 19.3 Run `make lint` and web tests
- [x] 19.4 Open PR

## Phase 20 — web admin chat (branch: feat/web-genagents-admin-chat)

- [x] 20.1 Add English and pt-BR copy
- [x] 20.2 Implement the admin chat page: chat list, messages, collapsible tool activity, message box, "Stop", polling
- [x] 20.3 Run `make lint` and web tests
- [x] 20.4 Open PR

## Phase 21 — web gentasks additions (branch: feat/web-gentasks-bots)

- [x] 21.1 Add English and pt-BR copy
- [x] 21.2 Implement the assignee picker "Bots" group and bot avatar, the board repository field in GitHub settings, the "Bot conversation" panel, and the "Bot review" button with reviewer picker, all hidden when genagents is disabled
- [x] 21.3 Run `make lint` and web tests
- [x] 21.4 Open PR

## Phase 22 — release, installer, docs (branch: chore/runner-release-docs)

- [x] 22.1 Write tests for the installer listing genagents and for release artifact names including `genwork-runner`
- [x] 22.2 Build and publish `genwork-runner` for linux/darwin amd64/arm64 and the `genwork-sandbox` image in the release workflow through `make build` (rule 75)
- [x] 22.3 Add `runner/` to the constitution component table and a sentence to rule 84 stating that the runner, outside the stack, may use its own machine's container engine for sandboxes; write `runner/README.md` (install, container engine (rootless recommended), custom sandbox images, directory mode, register, account sign-in, GitHub credentials, safety notes) and the genagents section of the main README; add `.env.example` placeholders
- [x] 22.4 Run `make lint`, `make test`, and `make build`
- [x] 22.5 Open PR

## Phase 23 — gates and manual verification

- [ ] 23.1 Run `make lint`, `make test`, and `make build` on `main` with every phase merged
- [ ] 23.2 Manual verification with the developer (rule 41): link two Claude accounts (one subscription on a runner, one API key) and a Codex account; create a dev, a reviewer, and an assistant bot with skills and an extra MCP server; set a board repository; assign a task to the dev bot → a sandbox container starts and is removed at the end, status updates in the panel, reply mid-run, pull request opened citing the task and linked on the card; from inside a run, confirm the bot cannot see other account profiles or the GitHub token; press "Bot review" → GitHub review with comments and summary in the panel; admin chat as admin reads tasks and host metrics and creates a task; member cannot see the admin chat; disable genagents → bot UI disappears and gentasks keeps working; pt-BR copy; dark and light themes
- [ ] 23.3 Archive the change after sign-off
