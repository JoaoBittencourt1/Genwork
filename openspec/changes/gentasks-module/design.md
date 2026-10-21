## Context

Core (`core/`) has auth, users, roles with permissions seeded by migration, invites, and discovery. Core issues Ed25519 JWTs whose claims are `sub` and the session token, and it publishes `GET /api/core/v1/auth/jwks`. Routes are registered through `platform.Router`, which panics on a route without a policy. Errors use the `{"error":{"code","fields"}}` body.

What does not exist yet: `modules/`, `sdk/`, a gateway, a module registry, a capabilities endpoint, an endpoint that lists users, and projects or workspaces. The web app proxies `/api` straight to core in development. The CLI runs `docker compose` against `deploy/docker-compose.yml` and fills `JWT_SIGNING_KEY` on first run.

The gateway technology (Caddy) was chosen by the developer for this change. The event bus is still an open decision and nothing here needs it.

Ports already in use: core `8484`, web dev `8485`, Mailpit `8486`, Postgres `5484`. This change adds gateway `8487`. Gentasks publishes no host port (rule 57).

## Goals / Non-Goals

**Goals:**
- Build a module contract that later modules reuse unchanged: manifest, registration, token validation, a stubbed core for tests, an isolated schema, and gateway routing.
- Ship gentasks as a working task board, end to end, in the web app.
- Deliver one component per phase, each phase green on its own (rules 44, 48, 63).

**Non-Goals:** see the proposal. At the design level there is also no shared Go code between core and `sdk/go`, and no generic plugin loading in the web app. Module UIs ship in the bundle (rule 58).

## Decisions

### 1. `module.yaml` shape and the registration payload (rules 7, 51)

```yaml
id: gentasks
version: 0.1.0
core: ">=1.0.0 <2.0.0"
apiPrefix: /api/gentasks/v1
requires: []                      # [{ id, version }]
permissions:
  - name: gentasks.board.read
    defaultRoles: [admin, member]
  - name: gentasks.board.manage
    defaultRoles: [admin, member]
  - name: gentasks.task.write
    defaultRoles: [admin, member]
  - name: gentasks.task.delete
    defaultRoles: [admin, member]
events: { publishes: [], consumes: [] }
```

The module embeds its own `module.yaml` with `//go:embed`. `sdk/go/manifest` parses and validates it: semver version, valid range, namespaced permissions, and an API prefix of the form `/api/<id>/v<N>`. The prefix is not tied to the module's own major version, because rule 61 has a module serve `/v1` and `/v2` side by side during a deprecation window. The registration body is the parsed manifest serialized to JSON. No Go code repeats these facts, and the module refers to its permission names through constants that the module's tests check against the manifest.

Core's API version becomes one semver constant, `1.0.0`, in `core/internal/discovery`. `/.well-known/genwork` keeps exposing `v1`, now derived from that constant's major version. Range checks use `github.com/Masterminds/semver/v3`; the standard library and `x/mod/semver` have no range support.

### 2. Registration transport and trust

- `PUT /internal/v1/modules/{id}` on core, authenticated with `GENWORK_MODULE_SECRET`. Core keeps only a SHA-256 of the secret in memory and compares in constant time (rule 67). All modules in an installation share one secret. Per-module secrets would need the installer to distribute them, which is out of scope.
- Modules reach core at `CORE_INTERNAL_URL` (`http://core:8484` on the compose network), not through the gateway, because the gateway blocks `/internal/`.
- The module's `serve` command starts its HTTP server, then registers in a goroutine with exponential backoff (1 s, capped at 30 s). A `409` (incompatible or missing requirement) is fatal: the process logs it and exits non-zero. Requests are served while registration is pending. Capabilities simply do not list the module until it registers.
- *Alternative considered:* core pulls `module.yaml` from each container. Rejected, because core would then need to know every module's address, and that conflicts with rule 52.

### 3. Core registry storage

New migration (never editing applied ones, rule 20):
- `modules(id, token, name text unique, version, core_range, api_prefix, requirements jsonb, enabled bool, registered_at)`. `name` holds the manifest `id`, because the column `id` is the internal key (rule 14). Requirements are a JSON array of `{ id, version }` rewritten on every registration: they mirror the manifest and are not domain records, so no rows are ever deleted for them.
- `permissions` gains `module_id bigint null → modules(id)` (null means a core permission) and `retired_at timestamptz null`

Registration runs in one transaction behind an advisory lock on the whole registry, because requirement checks read other modules. It upserts the module, inserts unknown permissions and grants them to `defaultRoles` right away (this is the "grant once" rule, because the grant happens only on insert), revives retired permissions that reappear with their old grants kept, and sets `retired_at` on permissions missing from the manifest. The role loader in `access` ignores retired permissions. `enabled` is `true` on first registration and is not changed by later ones. The CLI's enable/disable command (rule 56) will own it later.

The `core.user.read` seed belongs to the user directory (phase 2).

Feature packages: `core/internal/modules` (registration handler, service, repository, module credential authenticator) and `core/internal/capabilities` (read-only endpoint). The capabilities handler depends on a small `EnabledModuleLister` interface rather than on the module service (rule 9). Registration uses a new router policy, `platform.ModuleCredential`, so the route declares its guard like every other route (rule 18).

### 4. Permissions claim and local authorization

`auth.AccessClaims` gains `Permissions []string json:"permissions"`, filled from the role when the token is issued and refreshed. Modules authorize from the claim, so they never call core per request.

- *Trade-off:* a revoked session or changed grant keeps working in modules for up to 15 minutes, until the access token expires. Core itself keeps its per-request session check. This matches rule 69's short-lived tokens and is documented in `module-authentication`.
- *Alternative considered:* a token introspection endpoint called by modules, with caching. Rejected for now: it adds a network hop per request and becomes a single point of failure for every module.
- The web app already reads permissions from `/me`. That stays the client's source, and the claim is for servers only.

### 5. `sdk/go` layout

Separate Go module `github.com/JoaoBittencourt1/Genwork/sdk/go`:

```
sdk/go/
  manifest/      ModuleManifest, ParseManifest, validation
  registration/  RegistrationClient (PUT /internal/v1/modules/{id}, backoff)
  tokens/        TokenVerifier (JWKS cache, kid refetch ≤ 1/min, EdDSA, iss/aud/exp), Principal
  httpapi/       Router with mandatory policies (Authenticated, RequirePermission), WriteJSON/WriteError/DecodeJSON matching core's error body
  coreclient/    UserDirectoryClient (GET /api/core/v1/users/{token}, forwarding the caller's bearer)
  coretest/      fake core: in-memory key pair + JWKS server, token minting with chosen permissions, user directory stub, registration recorder
  coretest/fixtures/  recorded core responses (JWKS, user, user_not_found, capabilities, registration)
```

`httpapi.Router` mirrors core's policy router, but it authorizes from token claims instead of database roles and sessions. Core keeps its own router because its policy checks need its database. Sharing one router would make core import the SDK, and core's Docker build context cannot reach `sdk/`.

Contract fixtures are kept honest from both sides. Core tests (`core/tests/app`) decode core's real responses and compare their shape against the files in `sdk/go/coretest/fixtures/`, which they read through a relative path. The SDK's fakes serve those same files (rule 62).

### 6. Gentasks layout (rules 4, 50, 82)

```
modules/gentasks/
  module.yaml  Dockerfile  go.mod (replace sdk/go => ../../sdk/go)
  cmd/gentasks/main.go               calls command.Run
  internal/
    command/      serve, migrate
    platform/     config (env, fail fast), DB pool, board lock, error codes
    access/       permission names, checked against module.yaml by a test
    boards/       boards + columns (columns only exist within a board's workflow)
    labels/       labels and the task ↔ label association
    tasks/
    boardview/    the full board read (columns, active tasks, labels); separate so tasks can import boards without a cycle
    boardtest/    fixtures for tests (rule 26)
    app/          composition root, route registration
  migrations/     goose SQL, embedded
  tests/          mirrors internal/ without the prefix (same convention as rule 71)
```

The Dockerfile build context is the repository root, so the `replace` directive resolves. It copies only `sdk/go` and `modules/gentasks`.

Configuration: `GENTASKS_DATABASE_URL`, `GENTASKS_HTTP_ADDR`, `CORE_INTERNAL_URL`, `GENWORK_MODULE_SECRET`, `GENWORK_PUBLIC_URL` (expected issuer and audience). A missing value fails startup (rule 1).

### 7. Gentasks data model (rules 14, 22, 53)

The `gentasks` schema is owned by the `gentasks` role. Public tokens are generated by the service (`platform.NewPublicToken`, 16 random bytes as base64url, the same format as core). Core's `public_token()` is out of reach, and the `gentasks` role must not depend on extensions installed outside its own schema. `gentasks migrate` creates the schema only when it is missing, because in an installation the CLI provisions it and the role may not create schemas.

- `boards(id, token, name, created_by text, created_at, updated_at, deleted_at)`
- `board_columns(id, token, board_id, preset text null check in (backlog,todo,done), name text null, position int, created_at, updated_at, deleted_at)` with check `preset IS NOT NULL OR name IS NOT NULL`. The table is named `board_columns` to stay clear of SQL's `column` keyword and of `information_schema.columns`.
- `labels(id, token, board_id, name, color text check in palette, created_at, updated_at, deleted_at)` with unique `(board_id, lower(name)) WHERE deleted_at IS NULL`
- `tasks(id, token, board_id, column_id, title, description null, assignee_user_token null, urgency text check in (none,low,medium,high,urgent) default 'none', position int null, archived_at null, created_by text, created_at, updated_at, deleted_at)`. `position` is null exactly when the task is archived or deleted, which a check constraint enforces.
- `task_labels(id, token, task_id, label_id, unique(task_id, label_id))`. **Documented physical-delete exception (rule 22):** rows are hard-deleted when a label is detached from a task or the label is deleted. A row is an association, not a domain record, and the task and label themselves are soft-deleted.

User tokens (`created_by`, `assignee_user_token`) are opaque text with no foreign key, because they belong to core (rule 53). Urgency, preset, and color are Go typed constants (rule 15) mirrored by SQL checks.

### 8. Ordering

Positions are dense zero-based integers per column (tasks) and per board (columns). Every write that changes order (create, move, archive, unarchive, delete, column move or delete) runs in one transaction that first takes `SELECT … FROM boards WHERE id = $1 FOR UPDATE`. It then shifts the affected range with one `UPDATE … SET position = position ± 1 WHERE …` and sets the moved row. Uniqueness of `(column_id, position)` is not enforced by an index, because shifting rows would violate it mid-statement. The board lock serializes order changes instead, and tests assert that positions stay contiguous after concurrent moves.

- *Alternative considered:* fractional or lexicographic keys, which need one-row updates and no lock. Rejected for now: boards are small, lock contention is per board, and dense integers keep the API's `position` index meaningful to every client without a client-side key algorithm.

### 9. Reading a board without N+1 (rule 21)

`GET /boards/{token}` runs four queries regardless of size: board, columns, active tasks ordered by `(column position, task position)`, and the label links joined with labels for those task ids. The service assembles the response. Assignee names are not included. Clients resolve them through core's user directory, once per board view.

### 10. Assignee validation

`tasks.TaskService` depends on an `AssigneeVerifier` interface. The composition root wires `coreclient.UserDirectoryClient`, which forwards the caller's bearer token to `GET /api/core/v1/users/{token}` over `CORE_INTERNAL_URL` with a 3 s timeout. A `404` maps to a validation error, and a network error or `5xx` maps to `503 core_unavailable`. Tests use `coretest` (rule 25).

### 11. Gateway (Caddy)

`deploy/Caddyfile`, with the upstreams from environment placeholders (rule 1):

```
:{$GATEWAY_PORT}
respond /internal/* 404
reverse_proxy /api/core/* /.well-known/* {$CORE_UPSTREAM}
reverse_proxy /api/gentasks/v1/* {$GENTASKS_UPSTREAM}
respond 404
```

Adding a module means adding a route line. The installer will generate this file later (rule 64). Caddy replaces any incoming `X-Forwarded-For` with the peer address by default. Core's `TRUSTED_PROXY_CIDRS` is set to the compose network subnet, which the compose file pins in `networks:` because a subnet is not an environment-specific secret. The Vite dev proxy target becomes `http://localhost:8487`.

### 12. CLI provisioning (rules 53, 64)

`genwork db|api|up` first fill `GENWORK_MODULE_SECRET` and `GENTASKS_DB_PASSWORD` in `deploy/.env`, reusing the existing first-run key filling. After Postgres is healthy, they run an idempotent SQL script with `docker compose exec postgres psql`:

- `CREATE ROLE gentasks LOGIN` if it is missing, then `ALTER ROLE … PASSWORD`
- `CREATE SCHEMA IF NOT EXISTS gentasks AUTHORIZATION gentasks`
- `REVOKE ALL ON SCHEMA public FROM gentasks`

The password goes through a `psql` variable, so it never appears in the command line or logs (rule 67). `GENTASKS_DATABASE_URL` in `.env.example` points at the role. The script's SQL lives in a dedicated file under `cli/internal/` that names the module, as a first step toward per-module provisioning driven by `module.yaml`.

### 13. Web structure (rules 72–74, 58, 59)

- `shared/datasource/capabilities.ts`, `shared/domain/capabilities.ts` (`isModuleEnabled`), and a `CapabilitiesProvider` in `shared/components/`, loaded after sign-in. The sidebar entry and the route guard read it.
- `app/gentasks/datasource/`: `boardsApi.ts`, `columnsApi.ts`, `labelsApi.ts`, `tasksApi.ts`, all through `apiRequest`, plus a `usersApi.ts` call to the core directory placed in `shared/datasource/` because it is core.
- `app/gentasks/domain/`: types, `urgency.ts` (order, i18n key), `columnName.ts` (preset label key versus custom name), `ordering.ts` (pure optimistic move for columns and tasks, and rollback), `filters.ts`, `taskPatch.ts` (diff into a minimal `PATCH` body), `permissions.ts`, `labelPalette.ts` (palette to CSS variable name), and `errors.ts` (API code to message key).
- `app/gentasks/components/`: board, column, card, task sheet, label manager, filters, archived list, and query hooks.
- Pages: `BoardsPage.tsx` (`/tasks`) and `BoardPage.tsx` (`/tasks/$boardToken`).
- Drag and drop with `@dnd-kit/core` and `@dnd-kit/sortable`: accessible keyboard sensors, no DOM-measurement hacks, and actively maintained. *Alternatives:* `react-beautiful-dnd` (deprecated, rule 34) and native HTML5 drag-and-drop (no keyboard support, so it fails the spec).
- Palette colors are new theme variables `--label-<color>` and urgency variables in `web/src/index.css`, defined for dark and light themes (rule 81).
- The shadcn components `dialog`, `textarea`, and `alert-dialog` are added with `npx shadcn@latest add` (rule 73). Assignee, urgency, label, and filter pickers reuse `select` and the checkbox items of `dropdown-menu`, so no other component is needed (rule 77). The CLI still rewrote `cn` to the npm package of the same name even though the alias resolves; the generated files are corrected to `@/shared/lib/utils` and the stray package is removed.
- Task dragging is disabled while a filter is active: positions computed against a filtered column would not match the server's. Each card and column has a dedicated drag handle, so Enter on a card opens it instead of starting a drag.

### 14. Build and CI (rule 75)

The Makefile gains `lint-sdk`, `test-sdk`, `lint-gentasks`, `test-gentasks` (needs `TEST_DATABASE_URL`, and migrates into a throwaway schema like core), and `build-gentasks`. `lint`, `test`, and `build` include them. The `ci.yml` path filters gain `sdk/**` and `modules/gentasks/**`. Gentasks CI also runs when `sdk/**` changes, and core CI does not run it (rule 62, "core alone").

## Risks / Trade-offs

- [Permissions in the token go stale for up to 15 minutes in modules] → Short token lifetime (rule 69). Documented in the spec. Introspection can be added later without changing module routes.
- [One shared module secret: a compromised module could register as another] → Registration is reachable only on the internal network, and per-module secrets arrive with the installer. Noted in the installer change.
- [Hand-written Caddyfile routes drift from `module.yaml` API prefixes] → A gentasks test asserts that the Caddyfile contains its `apiPrefix`. The installer will generate the file later.
- [Dense positions under a board row lock serialize all reorders on one board] → The work per transaction is small. Revisit if real boards show contention.
- [Contract fixtures drift from core] → Core tests compare live responses with the fixture files, so drift fails core CI.
- [`replace` directive ties gentasks to the in-repo SDK] → Intended for the monorepo. SDK tagging and versioned imports come with the first independent module release.
- [Wide change spanning six components] → Nine single-component phases, each green on its own (see `tasks.md`).

## Migration Plan

1. Core phases deploy first. Without modules nothing changes for users, and capabilities return an empty list (rule 49).
2. The SDK has no runtime footprint.
3. Gentasks, the gateway, and the CLI land together in the local stack. Developers rerun `genwork up`, which fills the new `.env` values and provisions the role. They must update `web/.env` so `DEV_API_PROXY_TARGET` points at the gateway, and the CLI prints a hint when the value still points at core.
4. Web phases ship last and stay hidden until capabilities list gentasks.

Rollback: each phase is its own PR and can be reverted. Core migrations only add columns and tables. Dropping gentasks means stopping its container and leaving its schema in place (rule 56).
