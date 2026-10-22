## Context

See proposal.md for the motivation. The current state that shapes the design:

- **Versions are scattered.** Core gets `command.Version` through ldflags (default `dev`). Each `module.yaml` declares its own version (gentasks `0.4.0`, genmonitor `0.1.0`), and the SDK parses it with `KnownFields(true)` and strict semver. Core's module registry parses registered versions with `semver.MustParse`. `genwork-mcp` has a constant `0.1.0`. There are no git tags and no releases.
- **The stack is development-only.** `deploy/docker-compose.yml` has `build:` on every Go service, tags images `:dev`, and runs Mailpit. The CLI finds its workspace by walking up to `deploy/docker-compose.yml`, reads the module catalog from `modules/*/module.yaml`, and renders `deploy/Caddyfile` from it. The gateway answers `404` for anything that is not `/api/core`, a module prefix, or `/.well-known`. The web app is served only by Vite.
- **MCP is local.** `genwork-mcp` serves stdio only. It signs in as a `native` session and calls the gateway through an `AuthorizedAPIClient` fed by an `AccessTokenSource`. Its go-sdk (v1.8.0) already ships `auth.RequireBearerToken`, `auth.ProtectedResourceMetadataHandler`, and a Streamable HTTP handler that binds each session to the `TokenInfo.UserID` that created it.
- **Core already does most of what OAuth needs.** It issues Ed25519 JWTs (issuer = public URL, audience `genwork`, effective permissions, at most 15 minutes) and runs sessions with single-use refresh rotation and reuse revocation. Modules verify tokens with `sdk/go/tokens`.
- **The repository is public**, so release assets and GHCR images can be read without credentials.

## Goals / Non-Goals

**Goals:**
- One reproducible artifact per version, installable on a server with Docker and nothing else.
- Updates that an admin can start from the browser, that never leave the instance half-updated, and that share one code path with the CLI.
- An assistant connects with a URL and a browser sign-in. No binary is installed and no tool definition is duplicated.
- The development loop (`genwork up` in a clone) is unchanged.

**Non-Goals:** see proposal.md. At design level, also out of scope: high availability of the MCP server (one replica, in-memory sessions), CORS on the OAuth endpoints (browser-based MCP clients), and cleanup of unused OAuth client registrations.

## Decisions

### 1. One platform version in a root `VERSION` file (amends rules 50 and 51)
`VERSION` holds the platform version (e.g. `0.5.0`) and is its single owner (rule 7). Each component reads it differently:
- **Makefile:** reads `VERSION` and passes `-ldflags -X …Version=$(VERSION)` to every Go binary.
- **Dockerfiles:** take `ARG VERSION` and do the same.
- **Builds without ldflags** (`go run`, tests): fall back to `0.0.0-dev`, which is valid semver, so module registration keeps working.
- **Modules:** `sdk/go/manifest.ParseManifest(data, version)` receives the build version. `module.yaml` loses `version`, so a manifest that still declares one fails on the unknown field.
- **Core:** keeps reporting the version through `/.well-known/genwork`.
- **MCP and CLI:** `Version` becomes an injected variable.

`requires[].version` ranges stay in manifests. With one version they are always satisfied, but removing them would change core's registry contract for no gain now.

The first platform version is **`1.0.0`**, the first release published for installation. It is higher than any version a component registered before, so no registered version goes backwards.

Alternatives considered:
- Per-component versions plus a release manifest. This respects the current rule 50, but the developer chose a single version.
- `git describe`. Rejected because a deploy bundle has no git history.

### 2. Compose split: images by default, builds in a dev override
`deploy/docker-compose.yml` references images only, as `${GENWORK_IMAGE_REGISTRY}/genwork-<name>:${GENWORK_VERSION:?}`. A new `deploy/docker-compose.dev.yml` adds `build:` (with `VERSION` as a build arg), replaces each image with a local `genwork-<name>:dev`, adds Mailpit, and makes core depend on it. The CLI passes both files in a repository checkout and only the base file in a release installation.

- `GENWORK_IMAGE_REGISTRY` lives in `.env.example` (rules 1 and 3).
- `GENWORK_VERSION` is never stored in `.env`. The CLI reads it from `VERSION` and puts it in the environment of every compose call, so `VERSION` stays the only owner. Running compose by hand without the CLI fails loudly on `:?` (rule 1).
- `name: genwork` stays, so a release installation reuses the `genwork_postgres-data` volume of a checkout on the same machine.

Alternative considered: a single file with both `image:` and `build:`. Rejected because `docker compose up` might build on a server, and a dev `up --build` would overwrite release tags.

### 3. The gateway image carries the web bundle (rule 65; closes the gateway open decision in favor of Caddy)
`deploy/gateway/Dockerfile` is multi-stage: Node runs `npm ci && npm run build` in a build stage, then the runtime stage copies `dist/` into `/srv/web` on `caddy:2.10-alpine`. No Node process runs in production.

The CLI-rendered Caddyfile gains three routes:
- `/mcp` and `/mcp/*` go to `{$MCP_UPSTREAM}`.
- `/.well-known/oauth-protected-resource*` also goes to `{$MCP_UPSTREAM}`. This route comes before `/.well-known/*`, so the MCP server owns its resource metadata (rule 7).
- The catch-all serves `/srv/web` with `try_files {path} /index.html`, `Cache-Control: no-cache` on `index.html`, and immutable caching on `/assets/*`.

In development, the dev override keeps the plain Caddy image. `/srv/web` is empty there, so the catch-all answers `404` as it does today, and Vite serves the app and proxies `/api`, `/.well-known`, and now `/mcp`.

Caddy has been the gateway since the module PRs. The constitution still lists the gateway as an open decision, so this change closes it, pending the developer's approval.

### 4. Release workflow and make targets (rule 75)
`.github/workflows/release.yml` runs on `v*.*.*` tags:
1. fail unless the tag equals `v$(cat VERSION)`;
2. run `make lint`, `make test` (with the Postgres service), and `make build`;
3. log in to GHCR with `GITHUB_TOKEN` (`packages: write`);
4. run `make release-images`;
5. run `make release-artifacts`;
6. run `gh release create v$VERSION --generate-notes dist/*`.

Every build and publish step is a make target. The workflow only orchestrates.

- **`make release-images`:** runs `docker buildx build --platform linux/amd64,linux/arm64 --push` for core, gentasks, genmonitor, gateway, mcp, and updater, each tagged `$(VERSION)`. The Go Dockerfiles build on `--platform=$BUILDPLATFORM` and cross-compile with `GOOS`/`GOARCH` from `TARGETOS`/`TARGETARCH`, so arm64 needs no emulation.
- **`make release-artifacts`:** writes into `dist/`:
  - `genwork_<v>_<os>_<arch>.tar.gz` and `genwork-mcp_<v>_<os>_<arch>.tar.gz`, plus `.zip` for Windows `genwork-mcp`;
  - `genwork-bundle_<v>.tar.gz`, laid out like the repository (spec "Deploy bundle layout");
  - `install.sh`, rendered from `deploy/install.sh.tmpl` with the release repository taken from `GITHUB_REPOSITORY`, so no host is hardcoded in source (rule 1);
  - `checksums.txt`.

Release notes come from GitHub's generated notes over the merged PRs.

The developer cuts a release in `RELEASING.md` order: a PR that bumps `VERSION`, then `git tag v<version> && git push origin v<version>`.

### 5. Workspaces: repository checkout or release installation
`commands.Workspace` gains `Kind`: `repository` when `web/package.json` exists, otherwise `release`. `FindWorkspace` first honors `GENWORK_HOME`, then walks up from the current directory as today. Because the bundle mirrors the repository layout (`VERSION`, `deploy/`, `modules/*/module.yaml`), the module catalog, `.env` handling, and Caddyfile rendering work unchanged in both kinds. `EnsureEnvFiles` stops creating `web/.env` in a release installation. It upserts `GENWORK_HOME` in `deploy/.env` to the workspace's absolute path on every run, because the updater mounts that path (decision 8).

The release channel settings live in `.env.example`:
- `GENWORK_RELEASE_REPOSITORY`;
- the existing `GITHUB_API_URL`, reused (rule 6);
- `GENWORK_IMAGE_REGISTRY`.

`genwork install`:
1. refuses when `.env` and `genwork.yaml` already exist;
2. with `--from-checkout <path>`, first copies that checkout's `deploy/.env` and `deploy/genwork.yaml`;
3. fills secrets, including the new `GENWORK_UPDATER_SECRET`;
4. asks for `GENWORK_PUBLIC_URL`, `GENWORK_INSTANCE_NAME`, and optionally SMTP, warning when SMTP still points at `mailpit`;
5. shows the module checklist;
6. pulls the images and runs `up -d --wait`;
7. enables the modules, provisioning module databases through the existing path.

### 6. Release channel client (`cli/internal/releases`)
`ReleaseChannel` is an interface (rule 9) with two methods:
- `FindLatest(ctx) (Release, error)` returns version, notes URL, publication date, and asset URLs. It uses `GET {GITHUB_API_URL}/repos/{repo}/releases/latest`, which already skips drafts and pre-releases.
- `Download(ctx, asset, destination)` downloads with size limits and timeouts.

`VerifyChecksums` checks every downloaded file against `checksums.txt` before anything is extracted. Downloads land in `<home>/.genwork/staging/<version>/`. Tests use recorded GitHub API fixtures and an `httptest` server (rules 25 and 26).

### 7. One update engine for the CLI and the updater (`cli/internal/updates`)
`UpdateRunner` executes ordered steps. Each step records its name in `deploy/update-status.json`, written atomically. That file is the single owner of the update status (rule 7): the CLI prints it and the updater serves it.

A lock (`flock` on `deploy/.update.lock`) serializes the CLI and the updater. Docker access goes through the existing `CommandRunner`, so tests fake it.

The steps:
1. **`download`:** find the latest release, refuse `no_newer_release`, download the bundle and `checksums.txt`, verify, and extract into staging.
2. **`pull`:** `docker compose -f <staged compose> pull` with `GENWORK_VERSION=<target>`. The running stack is untouched. A failure stops here with `rolledBack: false`.
3. **`backup`:** `docker compose exec -T postgres pg_dump -Fc` into `backups/<timestamp>-<old version>.dump`.
4. **`swap`:**
   1. copy the current `deploy/docker-compose.yml`, `deploy/.env.example`, `deploy/.env`, `deploy/Caddyfile`, `VERSION`, and `modules/*/module.yaml` into `.genwork/previous/`;
   2. copy the staged bundle over the installation, except `VERSION`;
   3. append new `.env` keys and fill secrets (existing `updateStackEnv` and `EnsureModuleSecrets`);
   4. re-render the Caddyfile.
5. **`migrate`:** `up -d --wait --wait-timeout 300 --remove-orphans` with every service except `updater`. The `*-migrate` services run before their dependents through `depends_on: service_completed_successfully`, so a failed migration fails the `up`.
6. **`verify`:** poll `/.well-known/genwork` until it reports the target version, for up to 2 minutes. The updater uses `GATEWAY_INTERNAL_URL`. The host CLI uses `http://localhost:${GATEWAY_PORT}`, like `printEndpoints` does today.
7. **Finish:** write `VERSION`, mark `succeeded`, and prune the backups to the newest five.

Rollback runs on any failure in steps 4 to 6:
1. restore `.genwork/previous/`;
2. `stop` every service except Postgres;
3. `pg_restore --clean --if-exists --single-transaction` from this update's dump;
4. `up -d --wait` with the old `GENWORK_VERSION`, whose images are still local;
5. verify the old version.

When rollback succeeds, the status is `failed` with `rolledBack: true`. When it fails, the status has `rolledBack: false`, and the CLI prints the dump path and the restore commands.

Only writes made between the snapshot and the failure are lost. Pulling before the backup keeps that window short.

After success, the host CLI replaces its own binary (download, verify, write to a temp file next to it, rename). If the directory is not writable, it prints instructions. A host binary older than the installation's `VERSION` warns on every command.

### 8. The updater container (new rule 84)
`genwork updater` is a CLI subcommand. It serves an internal HTTP API on `UPDATER_HTTP_ADDR` and wraps the engine of decision 7:
- `GET /v1/status`;
- `POST /v1/check`, synchronous, with a 15-second timeout;
- `POST /v1/updates {version}`: answers `202` and runs the update asynchronously, refusing `409 version_mismatch` and `409 update_in_progress`.

Every request needs `Authorization: Bearer <GENWORK_UPDATER_SECRET>`, compared with `subtle.ConstantTimeCompare`. The secret is never logged (rule 67). The updater owns the "latest known release" fact, so core does not duplicate the version check.

The image is `deploy/updater/Dockerfile`: `docker:<pinned>-cli`, which brings the Docker CLI and the compose plugin, plus the `genwork` binary.

The compose service:
- mounts `${DOCKER_SOCKET_PATH}` and `${GENWORK_HOME}:${GENWORK_HOME}`. The installation sits at the same absolute path inside and outside, so relative bind mounts that the compose client resolves (e.g. `./Caddyfile`) point at the right host paths;
- uses `env_file: .env`, with no `environment:` overrides (rule 2);
- publishes no ports and has no gateway route;
- restarts `unless-stopped`.

**Replacing itself.** A container that recreates itself dies halfway through its own `compose up`. So after recording success, the updater starts a one-off container from the new image with `docker compose run -d --rm --no-deps updater genwork updater replace`. That container runs `docker compose up -d --no-deps --force-recreate updater` and exits. One-off containers are not the service's container, so they survive the recreate.

**Development checkout:** the dev override builds the updater from `cli/` too. It reports `updatable: false` with reason `development_installation`.

Alternatives considered:
- A systemd service on the host. It needs root setup outside compose, and a second lifecycle to install and update.
- Mounting the socket into core. That gives the public API root on the host.
- Routing the gateway straight to the updater, which would then verify JWTs. That puts a root-equivalent process on the public path.

### 9. Core relays updates (`core/internal/updates`; amends rule 49)
The feature has four parts:
- an `UpdaterClient` interface with an HTTP implementation, wired in `app.go` (rule 9);
- `UpdateService`;
- `UpdateHandler`, registering `GET /api/core/v1/system/update`, `POST /api/core/v1/system/update/check`, and `POST /api/core/v1/system/update`, each with `RequirePermission(core.system.update)` (rule 18);
- `StartUpdateRequest`, which validates `version` as strict semver at the boundary (rule 11), and an explicit `UpdateStatusResponse` (rule 13).

Updater error codes map to API errors: `version_mismatch`, `update_in_progress`, and `no_newer_release` become `409`. An unreachable updater, or one that answers `401`, becomes `503 updater_unavailable`.

Migration `00008_system_update_permission.sql` seeds `core.system.update` for `admin`. Config gains `GENWORK_UPDATER_URL` and `GENWORK_UPDATER_SECRET` (at least 22 characters), both required (rule 1). Core tests stub the updater with an `httptest` server and recorded status fixtures.

Core runs and passes its tests with zero modules (rule 49). The relay is generic and names no module (rule 52).

### 10. Web update card (`web/src/app/system/`)
The feature folder follows the rule 72 split:
- **`datasource/updates.ts`:** `findUpdateStatus`, `checkForUpdates`, and `startUpdate`, with error mapping.
- **`domain/updateCard.ts`:** a pure `deriveUpdateCardView(status, { lastPollError, unreachableSince, now })`. It returns `idle | checking | upToDate | available | confirming | updating | restarting | succeeded | failed | gaveUp | notUpdatable`, plus `shouldReloadApp` and `nextPollDelay`. Every decision lives here (rule 74).
- **`components/`:** `UpdateCard`, the update confirm dialog (it reuses `ConfirmDialog`), and `useUpdateStatus`, a TanStack Query hook that polls every 3 s while updating and treats network errors and `502`/`503` as `restarting`.

`HomePage` renders the card when `can(PERMISSIONS.systemUpdate)`. The success path calls `window.location.reload()`, so the browser loads the new bundle.

### 11. Core OAuth authorization server (`core/internal/oauth`)
**Data** (migration `00009_oauth.sql`):
- `oauth_clients`: `id`, `token` (the public `client_id`, rule 14), `name`, `redirect_uris text[]`, `created_at`, and `deleted_at`.
- `oauth_authorization_codes`: `id`, `token`, `code_hash bytea UNIQUE` (rule 67), `client_id`, `user_id`, `redirect_uri`, `code_challenge`, `scope`, `expires_at`, `used_at`, `session_id NULL`, and `created_at`.
- `sessions`: the `client_type` check gains `oauth`, plus a nullable `oauth_client_id`.
- `sessions.ClientType` gains `ClientOAuth` (rule 15). An OAuth session's device name is the client name.

**Endpoints:**
- `GET /.well-known/oauth-authorization-server` (public): metadata built from `PublicURL`.
- `POST /api/core/v1/oauth/register` (public, its own rate limit): RFC 7591.
- `GET /api/core/v1/oauth/authorization-requests` (authenticated): validates the query the consent page received and returns `{clientName, scope}`. A redirectable error comes back as `{redirectUrl}`, and a non-redirectable one as a plain `400`.
- `POST /api/core/v1/oauth/authorizations` (authenticated): the same parameters plus `decision: allow | deny`. Answers `{redirectUrl}`.
- `POST /api/core/v1/oauth/token` (public, rate limited): form-encoded, `authorization_code` and `refresh_token` grants.

**Validation lives once.** Validation of an authorization request is a single `AuthorizationRequest` type, used by both the preview and the decision.

**Token issuance reuses the login path.** A code exchange calls `sessions.Service.Start` with `ClientOAuth`, then the same token issuance that login uses (effective permissions). A refresh first checks that the session's `oauth_client_id` matches `client_id`, then uses `sessions.Service.Rotate`. Reuse revocation and expiry therefore stay in one place. A reused code revokes the session recorded on it.

**The token and registration endpoints deviate from the platform error envelope.** They answer in the RFC 6749 / RFC 7591 error format (`{error, error_description}`), and the token endpoint adds `Cache-Control: no-store`, because OAuth clients parse that shape. They are the only endpoints that deviate, and only for this protocol reason. Registration also ignores unknown client metadata, as RFC 7591 requires.

**`resource` (RFC 8707)** is accepted when it equals `<PublicURL>/mcp` and refused with `invalid_target` otherwise. The token audience stays `genwork`. The platform's own authorization server issues these tokens for the platform's audience, and the MCP server is a first-party facade over the same APIs. So forwarding the token to those APIs keeps every authorization decision on the user's own permissions, and no foreign token passes through.

Alternative considered: token exchange per tool call, a second audience-bound token for the APIs. It adds an endpoint and a round trip per call with no security gain inside one trust boundary.

**Routes.** `/oauth/authorize` is a web route, not an API route (rule 16 applies to REST). The existing `/.well-known/*` gateway route already reaches core.

### 12. MCP over Streamable HTTP (`mcp/internal/remote`)
`genwork-mcp serve-http` reads four settings:
- `MCP_HTTP_ADDR`;
- `GENWORK_PUBLIC_URL`, for the resource URL and the token issuer;
- `CORE_INTERNAL_URL`, for JWKS;
- `GATEWAY_INTERNAL_URL`, the API base for tool calls.

It builds an `http.ServeMux` with two parts:
- `/.well-known/oauth-protected-resource` and `/.well-known/oauth-protected-resource/mcp` use go-sdk's `ProtectedResourceMetadataHandler`.
- `/mcp` is `auth.RequireBearerToken(verifier, {ResourceMetadataURL})` wrapping `mcp.NewStreamableHTTPHandler(newSessionServer, …)`.

The verifier adapts `sdk/go/tokens.TokenVerifier` (rule 54): `mcp` gains the SDK dependency with the same `replace` as the modules. The adapter returns `TokenInfo{UserID: subject, Expiration}`, so go-sdk refuses a session used with another user's token.

`newSessionServer` builds one `mcp.Server` per session. Its `ToolsetSynchronizer` and toolsets get an `AuthorizedAPIClient` whose `AccessTokenSource` reads the bearer token of the current request from the context. The toolset code is unchanged. Synchronization runs when the session starts, and then lazily: a small wrapper resyncs when a request arrives more than `ToolsetSyncInterval` after the last sync. That replaces `SyncEvery`, which has no token to use between requests.

The new `mcp/Dockerfile` builds from the repository root, like the modules. Stdio `serve` is untouched.

### 13. Web assistant pages (`web/src/app/assistants/`)
- **`domain/connection.ts`:** `buildMcpUrl(origin)`, `buildClaudeCodeCommand(url)`, `buildClientConfigJson(url)`, and `isReachableFromInternet(origin)` (https and not localhost, loopback, or RFC 1918/4193).
- **`domain/authorizationRequest.ts`:** parses the consent query.
- **`datasource/oauth.ts`:** preview and decide.
- **`ConnectAssistantPage.tsx`:** route `/assistants`, with a sidebar entry for every signed-in user and a home card.
- **`AuthorizeAssistantPage.tsx`:** route `/oauth/authorize`, inside `RequireAuth` with the focused `AuthLayout`, not `AppShell`.

`RequireAuth` navigates to `/login?redirect=<path+search>`. `LoginPage` follows `safeRedirectTarget(raw)`, a new rule in `app/auth/domain`, which accepts only paths that start with a single `/`. Vite's dev proxy gains `/mcp`.

### 14. Constitution and docs
In this change:
- **Rule 49:** core also owns the OAuth authorization server for assistants, as part of auth, and the update relay.
- **Rule 50:** every component ships in the platform release under the version in `VERSION`.
- **Rule 51:** drops `version` from the manifest facts.
- **Rule 57:** the gateway also routes `/mcp` and the MCP resource metadata to the MCP server, and serves the web bundle on every other path.
- **Rule 64:** the installer also installs from releases and updates installations.
- **New rule 84:** only the updater mounts the Docker socket. It has no public route, accepts only core with the updater secret, and installs only checksum-verified releases of the configured channel.
- **Open decisions:** the gateway entry is removed (Caddy).
- **The component table:** `cli/` also runs as the updater container.

`README.md` gains install and update sections, and its "each component is versioned independently" sentence is replaced. `mcp/README.md` documents remote connection first. `RELEASING.md` is new.

### 15. One PR (exception to rules 42, 44, and 46)
The developer asked for everything in one PR. The phases in tasks.md still set the implementation and review order. Each phase starts with its tests and ends with lint and tests green. If the developer refuses the exception, each phase becomes its own branch and PR in table order.

## Risks / Trade-offs

- [The Docker socket is root-equivalent] → only the updater mounts it. It has no published port and no gateway route, requires the secret, accepts only "install the latest verified release", and never runs arbitrary commands. Rule 84 makes this binding.
- [Downtime during an update, typically under a minute after the pull] → the confirmation says so. Pulling before stopping anything keeps the window short.
- [Writes made during an update are lost if it rolls back] → the window runs only from the backup to the failure. A failed update is rare and announced.
- [GHCR packages start private] → a one-time manual step makes them public after the first release. `install.sh` reports `denied` with a hint.
- [GitHub API anonymous rate limit (60 requests per hour per IP)] → checks happen only on demand, and the status reuses the last result.
- [pg_dump time on large databases] → acceptable at the current scale. Streaming or incremental backups are a later concern.
- [In-memory MCP sessions are lost when the container restarts] → clients get `404` for an unknown session and re-initialize, as the transport specifies.
- [claude.ai and Claude Desktop need the instance reachable from the internet over HTTPS] → the connect page warns, and Claude Code still works on private networks.
- [Open redirect through the login return] → `safeRedirectTarget` accepts only single-slash relative paths, and the spec includes the attack scenarios.
- [Abuse of dynamic client registration] → it is rate limited per address, and each registration is tiny. Cleanup of unused clients is deferred.
- [The single PR is large] → phases keep it reviewable in order, and every phase leaves the gates green.

## Migration Plan

1. Merge the PR with `VERSION` set to `1.0.0`, then tag `v1.0.0`. The workflow publishes the release.
2. Make the GHCR packages public (one-time step).
3. Move staging from its clone:
   1. `genwork down` in the clone;
   2. `curl -fsSL <release>/install.sh | GENWORK_HOME=/opt/genwork sh -s -- --from-checkout <clone path>`;
   3. point the host's HTTPS proxy at the gateway port instead of the Vite port;
   4. check that `GENWORK_PUBLIC_URL` is the HTTPS address.

   The stack reuses the `genwork_postgres-data` volume.
4. Check the update path: publish `v1.0.1` with a trivial change, then update from the home card.
5. **Rollback of this change:** `genwork down` in `/opt/genwork`, then `genwork up` in the old clone. Migrations `00008` and `00009` only add rows, tables, and a nullable column, so older code ignores them.
