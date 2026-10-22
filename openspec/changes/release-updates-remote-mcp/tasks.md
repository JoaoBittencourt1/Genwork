### Phased delivery

**Exception to rules 42, 44, and 46, pending the developer's approval:** the developer asked for releases, updates, and remote MCP in **one PR**. Every phase below ships on the same branch, `feat/release-updates-remote-mcp`. The phases still set the implementation and review order, each one starts with its tests, and each one ends with lint and tests green. The PR is opened only after the last phase and the developer's manual verification (rule 41). If the exception is refused, each phase gets its own branch (`feat/release-updates-remote-mcp-<phase slug>`) and PR, in the order of the table.

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | Constitution amendments, single platform version (`VERSION`, ldflags, manifest, sdk) | — | — |
| 2 | Release images and artifacts: compose split, gateway image with web bundle, Dockerfiles, make targets, release workflow | 1 | — |
| 3 | CLI workspaces, release channel client, `genwork install` | 1, 2 | 5, 8 |
| 4 | Update engine, `genwork update`, updater service | 3 | 5, 8 |
| 5 | Core OAuth authorization server | 1 | 3, 4, 8 |
| 6 | Core update relay (`core.system.update`) | 4 (contract) | 5 |
| 7 | MCP over Streamable HTTP, MCP image, gateway routes | 2, 5 | 6, 8 |
| 8 | Web logic: login return, update and assistant datasources and domains (API stubbed with MSW) | — (contracts in specs) | 3–7 |
| 9 | Web UI: update card, connect page, consent page, sidebar, i18n | 6, 7, 8 | — |
| 10 | Docs, full gates, manual verification on staging, single PR | 1–9 | — |

Go phases follow rule 47's order (test → migration → model → repository → authorization → request type → service → handler → response type). Core tests live under `core/tests/` (rule 71), fixtures in dedicated files (rule 26), and no test reaches the network (rule 25).

## 1. Phase 1 — constitution and single platform version

- [x] 1.1 Write tests: `manifest.ParseManifest(data, version)` takes the build version and rejects a `version` key in `module.yaml`; the module registration request carries the build version; a build without ldflags reports `0.0.0-dev`, which registration accepts; `genwork version` and `genwork-mcp version` print the injected version
- [x] 1.2 Amend `constitution.md`: rules 49, 50, 51, 57, and 64, new rule 84, the component table, and the removal of the gateway open decision (design §14)
- [x] 1.3 Add the root `VERSION` file with `1.0.0`
- [x] 1.4 Change `sdk/go/manifest` to take the version from the caller, and update gentasks and genmonitor `serve` to pass their build `Version`
- [x] 1.5 Remove `version` from every `modules/*/module.yaml`, and update the CLI catalog and its fixtures
- [x] 1.6 Add an injectable `Version` to `cli` and `mcp` (default `0.0.0-dev`), and a `version` command in the CLI
- [x] 1.7 Make the Makefile read `VERSION` and pass `-ldflags -X` in the build targets
- [x] 1.8 Run `make lint` and `make test`

## 2. Phase 2 — release images and artifacts

- [x] 2.1 Write tests: the compose files render with `GENWORK_VERSION` set and fail loudly without it; the Caddyfile renderer emits `/mcp`, the protected-resource route before `/.well-known/*`, and the SPA catch-all with `try_files` and cache headers; `install.sh` rendering takes the repository from the environment; bundle assembly contains only the files in the spec
- [x] 2.2 Rewrite `deploy/docker-compose.yml` to use images (`GENWORK_IMAGE_REGISTRY`, `GENWORK_VERSION`), add `deploy/docker-compose.dev.yml` with `build:` and Mailpit, and add the new keys to `.env.example` (rules 1–3)
- [x] 2.3 Add `deploy/gateway/Dockerfile` (Node build stage, Caddy runtime with `/srv/web`) and update the Caddyfile template
- [x] 2.4 Make the core, module, and CLI Dockerfiles cross-compile with `BUILDPLATFORM` and take `ARG VERSION`
- [x] 2.5 Add `make release-images` and `make release-artifacts` (binaries, bundle, rendered `install.sh`, `checksums.txt`)
- [x] 2.6 Add `.github/workflows/release.yml` (tag matches `VERSION`, gates, GHCR login, make targets, `gh release create`)
- [x] 2.7 Run `make lint`, `make test`, and `make build`, then build the images locally with `docker buildx` for one platform

## 3. Phase 3 — CLI workspaces, release channel, install

- [x] 3.1 Write tests: `FindWorkspace` honors `GENWORK_HOME`, then walks up; workspace kind is `repository` with `web/package.json` and `release` without it; `EnsureEnvFiles` skips `web/.env` in a release installation and upserts `GENWORK_HOME`; `ReleaseChannel.FindLatest` against recorded GitHub fixtures (latest, none, rate limited, unreachable); `VerifyChecksums` accepts a match and reports the mismatching asset; `genwork install` refuses an existing installation, validates the public URL, supports `--from-checkout`, and never builds images
- [x] 3.2 Implement `Workspace.Kind` and `GENWORK_HOME` lookup; pass the right compose files per kind; make `genwork web` refuse in a release installation
- [x] 3.3 Implement `cli/internal/releases`: `ReleaseChannel` interface, GitHub implementation, download with limits, `VerifyChecksums`
- [x] 3.4 Implement `genwork install` (prompts and flags, secrets including `GENWORK_UPDATER_SECRET`, module checklist, pull, `up -d --wait`, enable modules)
- [x] 3.5 Write `deploy/install.sh.tmpl` (download CLI and bundle, verify, run `genwork install`)
- [x] 3.6 Run `make lint-cli` and `make test-cli`

## 4. Phase 4 — update engine, `genwork update`, updater

- [x] 4.1 Write tests with a fake `CommandRunner` and release fixtures: each step in order; `no_newer_release`; `checksum_mismatch` before the backup; pull failure leaves everything untouched; migration failure restores the files and the dump and reports `rolledBack: true`; rollback failure reports `rolledBack: false` with the dump path; new `.env` keys get generated secrets and existing values stay; backups pruned to five only after success; lock refuses `update_in_progress`; repository checkouts refuse with `development_installation`; self-replace writes atomically and prints instructions when the directory is not writable; updater API: `401` without the secret, constant-time compare, `202` async apply, `409` cases, status survives a restart
- [x] 4.2 Implement `cli/internal/updates`: `UpdateRunner`, step status file with atomic writes, lock, rollback
- [x] 4.3 Implement `genwork update` and `genwork update --check`, including binary self-replacement and the out-of-date warning
- [x] 4.4 Implement `genwork updater` (HTTP API, secret check, `replace` subcommand for self-replacement)
- [x] 4.5 Add `deploy/updater/Dockerfile` and the `updater` service in compose (socket and `GENWORK_HOME` mounts, no ports, no `environment:` block), and add the image to `make release-images`
- [x] 4.6 Run `make lint-cli` and `make test-cli`, and check CLI coverage stays ≥ 75% (rule 70)

## 5. Phase 5 — core OAuth authorization server

- [x] 5.1 Write tests in `core/tests/`: metadata document; registration (accepted, loopback `http`, insecure URI, too many URIs, no secret issued, rate limit); authorization request validation (unknown client and unregistered URI do not redirect; PKCE, scope, resource, and response type errors redirect with `state`); consent allow and deny; code single-use, expiry, and hash-only storage; token endpoint (exchange, wrong verifier, reused code revokes the session, refresh rotation, reused refresh revokes the session, refresh by another client, `no-store`, RFC 6749 error shape); OAuth sessions listed and revocable; deactivated member cannot exchange or refresh
- [x] 5.2 Write migration `00009_oauth.sql`: `oauth_clients`, `oauth_authorization_codes`, the `oauth` client type and `sessions.oauth_client_id`, with a `Down` that reverses it
- [x] 5.3 Add `sessions.ClientOAuth` and the session start with a client reference; add the `oauth` repositories (clients, codes)
- [x] 5.4 Implement `AuthorizationRequest` validation, `OAuthService` (register, preview, decide, exchange code, refresh), reusing sessions and token issuance
- [x] 5.5 Implement request types, `OAuthHandler`, and response types; register the routes with their authorization rules (rule 18) in `internal/app/app.go`; add the rate limits
- [x] 5.6 Run `make lint-core` and `make test-core`, and check core coverage stays ≥ 75%

## 6. Phase 6 — core update relay

- [x] 6.1 Write tests: `403` without `core.system.update` and the updater never called; status, check, and apply relayed with the updater stubbed by `httptest` and recorded fixtures; error mapping (`version_mismatch`, `update_in_progress`, `no_newer_release` to `409`; unreachable or `401` to `503 updater_unavailable`); version validated as semver at the boundary
- [x] 6.2 Write migration `00008_system_update_permission.sql`: seed `core.system.update` for `admin`
- [x] 6.3 Add `PermissionSystemUpdate` to `access`, and `GENWORK_UPDATER_URL` and `GENWORK_UPDATER_SECRET` to the core config (required, secret ≥ 22 characters)
- [x] 6.4 Implement `core/internal/updates`: `UpdaterClient` interface and HTTP implementation, `UpdateService`, `StartUpdateRequest`, `UpdateStatusResponse`, `UpdateHandler`; wire in `app.go`
- [x] 6.5 Run `make lint-core` and `make test-core`, and check core coverage stays ≥ 75%

## 7. Phase 7 — MCP over Streamable HTTP

- [x] 7.1 Write tests in `mcp/tests/`: `401` with the `WWW-Authenticate` header for a missing, invalid, and expired token; protected resource metadata at both paths; initialize and tool listing with a valid token; tools follow the user's permissions and enabled modules; resync after the interval and `tools/list_changed`; a request with another user's token on an existing session is refused; calls use the request's token and a refreshed token mid-session; stdio `serve` unchanged
- [x] 7.2 Add the `sdk/go` dependency to `mcp` (same `replace` as the modules) and review the `go.mod` diff (rule 77)
- [x] 7.3 Implement `mcp/internal/remote`: configuration, bearer verifier adapter, per-session server with a request-scoped `AccessTokenSource`, lazy toolset resync
- [x] 7.4 Add the `serve-http` command in `internal/command` and wire it in `cmd/` (rule 82)
- [x] 7.5 Add `mcp/Dockerfile`, the `mcp` service in compose, `MCP_UPSTREAM` and `MCP_HTTP_ADDR` in `.env.example`, and the image in `make release-images`
- [x] 7.6 Run `make lint-mcp` and `make test-mcp`, and check MCP coverage stays ≥ 75%

## 8. Phase 8 — web logic

- [x] 8.1 Add MSW handlers and fixtures in `web/src/test/` for the update endpoints and the OAuth preview and decision endpoints
- [x] 8.2 Write tests: `safeRedirectTarget` (relative paths accepted; `//host`, absolute URLs, and backslash tricks rejected); `deriveUpdateCardView` for every state in the spec (up to date, available, updating, restarting on network errors and `502`/`503`, succeeded with reload, failed with and without rollback, gave up after 15 minutes, not updatable); update datasource request shapes and error mapping; `buildMcpUrl`, `buildClaudeCodeCommand`, `buildClientConfigJson`, and `isReachableFromInternet` (https, localhost, loopback, private ranges); consent query parsing; OAuth datasource shapes and error mapping
- [x] 8.3 Implement `app/auth/domain/safeRedirectTarget.ts`, and make `RequireAuth` and `LoginPage` pass and follow the return path
- [x] 8.4 Implement `app/system/datasource/updates.ts` and `domain/updateCard.ts`, and add `PERMISSIONS.systemUpdate`
- [x] 8.5 Implement `app/assistants/domain/` and `datasource/oauth.ts`
- [x] 8.6 Run `make lint-web` and the web tests

## 9. Phase 9 — web UI

- [x] 9.1 Implement `app/system/components`: `UpdateCard`, the update confirmation (reusing `ConfirmDialog`), and `useUpdateStatus` with polling; render the card on `HomePage` for `core.system.update`
- [x] 9.2 Implement `ConnectAssistantPage` (`/assistants`) with copy buttons, per-client steps, and the reachability warning; add the sidebar entry and the home card
- [x] 9.3 Implement `AuthorizeAssistantPage` (`/oauth/authorize`) inside `RequireAuth` with `AuthLayout`; register the routes in `router.tsx`
- [x] 9.4 Add `/mcp` to the Vite dev proxy
- [x] 9.5 Add the pt-BR and English copy: card states, steps, error codes, connect page, consent page
- [x] 9.6 Add missing shadcn components with `npx shadcn@latest add` after checking the `web/components.json` aliases (rule 73), and review the dependency diff (rule 77)
- [x] 9.7 Run `make lint-web` and the web tests

## 10. Phase 10 — docs, gates, and verification

- [x] 10.1 Update `README.md` (install from a release, update, remote MCP), rewrite `mcp/README.md` to lead with the remote connection, and add `RELEASING.md`
- [x] 10.2 Update `.github/workflows/ci.yml` path filters for the new folders and `VERSION`
- [x] 10.3 Read the whole diff, including generated and scripted edits (rule 80)
- [x] 10.4 Run `make lint`, `make test`, and `make build`
- [ ] 10.5 Manual verification with the developer: install in a clean directory with `install.sh`, update from `1.0.0` to a test release from the CLI and from the home card (including a forced migration failure and rollback), and connect Claude Code and claude.ai to staging through `/mcp`
- [x] 10.6 After the developer's sign-off (rule 41), open the single PR in pt-BR, recording the exception to rules 42, 44, and 46

## 11. Additional changes made during verification

- [x] 11.1 Core grants a role a module permission when the module's manifest starts declaring that role (migration `00010_module_permission_default_roles.sql`), and genmonitor's `genmonitor.host.read` now defaults to `admin` and `member`, so the per-member module toggle can show the Server to members
- [x] 11.2 gentasks: the assignee avatar on a task card opens a menu that changes the assignee without opening the task
