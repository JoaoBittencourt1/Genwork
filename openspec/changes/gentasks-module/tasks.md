### Phased delivery

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | core: module registry (migration, registration endpoint, permission ownership and retirement) and capabilities endpoint | — | 2, 10 |
| 2 | core: user directory, `core.user.read` seed, `permissions` claim in access tokens | — | 1, 10 |
| 3 | sdk/go: manifest, registration client, token verifier, policy router, core client, `coretest` with recorded fixtures | 1, 2 merged | 10 |
| 4 | gentasks foundation: scaffolding, config, `module.yaml`, migrations, models, repositories, startup registration, Makefile/CI | 3 merged | 8, 10 |
| 5 | gentasks boards and columns API | 4 merged | 6, 7, 8, 9, 10 |
| 6 | gentasks labels API | 4 merged | 5, 7, 8, 9, 10 |
| 7 | gentasks tasks API (create, read, update, move, archive, delete, assignee check) | 4 merged | 5, 6, 8, 9, 10 |
| 8 | cli: secret generation and module database provisioning | — | 1–7, 10 |
| 9 | deploy: Caddy gateway, gentasks services in compose, env examples, Vite proxy target | 4, 8 merged | 5, 6, 7, 10 |
| 10 | web logic: capabilities and gentasks `datasource/` + `domain/` (API stubbed with MSW) | — (contracts in specs) | 1–9 |
| 11 | web UI: gentasks pages and components, sidebar entry, route guard, shadcn components, drag and drop, i18n, theme variables | 9, 10 merged | — |

Phases 1 and 2 add the core migrations `00005` and `00006` respectively. If phase 2 merges first, it takes `00005` and phase 1 renames its unmerged file before merging (rule 20 forbids editing only applied migrations). Go phases follow rule 47's order. Every phase stops for developer sign-off before its PR (rule 41).

## Phase 1 (branch: feat/core-module-registry)

- [x] 1.1 Write tests in `core/tests/modules` and `core/tests/capabilities`: registration with a valid secret, then a wrong or missing secret (`401`), path/manifest id mismatch (`400`), permission outside the namespace (`400`), unknown default role (`400`), incompatible core range and missing requirement (`409`), idempotent re-registration, default grants applied only on first sight, retired permission no longer granted and revived on reappearance; capabilities with zero modules and with gentasks; `401` without a token
- [x] 1.2 Write the migration: `modules` (with requirements as jsonb) and `permissions.module_id` / `permissions.retired_at`
- [x] 1.3 Add the semver core API constant in `discovery`, derive `/.well-known` `api` from it, and add `github.com/Masterminds/semver/v3` (review `go.mod`, rule 77)
- [x] 1.4 Implement the `modules` model, repository, request type (`RegisterModuleRequest` with validation), service (transaction + advisory lock), and handler on `PUT /internal/v1/modules/{id}` with a module-secret policy
- [x] 1.5 Make the `access` role loader ignore retired permissions
- [x] 1.6 Implement `capabilities` (finder interface, handler, `CapabilitiesResponse`) on `GET /api/core/v1/capabilities` (authenticated)
- [x] 1.7 Require `GENWORK_MODULE_SECRET` in core config, and document it in `deploy/.env.example`
- [x] 1.8 Run `make lint`, `make test`, `make build`
- [ ] 1.9 Developer sign-off, then open PR

## Phase 2 (branch: feat/core-user-directory)

- [x] 2.1 Write tests: list users ordered by name without deleted users or private fields, find by token, `404 user_not_found`, `403` without `core.user.read`; access token contains `permissions` after login and refresh, and drops a removed grant on the next refresh
- [x] 2.2 Write the migration seeding `core.user.read` for `admin` and `member`
- [x] 2.3 Add `List` and `FindByToken` to the users repository and service, plus `UserDirectoryResponse` and the handler routes with `RequirePermission(core.user.read)`
- [x] 2.4 Add `Permissions` to `AccessClaims` and fill it on issue and refresh
- [x] 2.5 Add `core.user.read` to the web `PERMISSIONS` map
- [x] 2.6 Run `make lint`, `make test`, `make build`
- [ ] 2.7 Developer sign-off, then open PR

## Phase 3 (branch: feat/sdk-go)

- [x] 3.1 Create the `sdk/go` Go module, and add `lint-sdk` / `test-sdk` to the Makefile and `sdk/**` to the CI path filters
- [x] 3.2 Record the contract fixtures from core (JWKS, user, `user_not_found`, capabilities, registration response) into `sdk/go/coretest/fixtures/`, and add a core test in `core/tests/app` that compares live responses with them
- [x] 3.3 Write tests for `manifest`: valid file, bad semver, bad range, permission outside the namespace, API prefix not matching the id or major version
- [x] 3.4 Write tests for `tokens` (valid, expired, wrong iss/aud, unknown key, `kid` refetch rate limit, JWKS unreachable → unavailable) and `httpapi` (route without a policy panics, `401`/`403`, error body shape)
- [x] 3.5 Write tests for `registration` (backoff on network errors, fatal on `409`) and `coreclient` (user found, `404`, timeout)
- [x] 3.6 Implement `manifest`, `tokens`, `httpapi`, `registration`, `coreclient`, and `coretest`
- [x] 3.7 Review `go.mod` (yaml.v3, golang-jwt, semver; rule 77)
- [x] 3.8 Run `make lint`, `make test`, `make build`
- [ ] 3.9 Developer sign-off, then open PR

## Phase 4 (branch: feat/gentasks-foundation)

- [x] 4.1 Scaffold `modules/gentasks` (`go.mod` with `replace` to `../../sdk/go`, `cmd/gentasks`, `internal/command` with `serve`/`migrate`, `internal/platform` config failing on missing variables, `Dockerfile` with the repo root as build context), embed `module.yaml`, and add `lint-gentasks` / `test-gentasks` / `build-gentasks` and CI filters
- [x] 4.2 Write tests: config fails on each missing variable; `module.yaml` parses and its permissions match the Go permission constants; migrations apply into a throwaway schema; repositories for boards, columns, labels, tasks, and task labels (create, find by token excluding deleted rows, positions, the case-insensitive unique label name with soft delete); the board lock serializes concurrent position shifts
- [x] 4.3 Write the migrations: `public_token()`, `boards`, `columns`, `labels`, `tasks`, `task_labels`, with checks and partial indexes (design §7)
- [x] 4.4 Implement models with typed constants (`Urgency`, `ColumnPreset`, `LabelColor`) and the repositories, including the batched board-read queries (design §9)
- [x] 4.5 Wire `serve`: HTTP server with `httpapi.Router`, `/healthz`, and background registration with core (fatal on `409`)
- [x] 4.6 Run `make lint`, `make test`, `make build`
- [ ] 4.7 Developer sign-off, then open PR

## Phase 5 (branch: feat/gentasks-boards-api)

- [x] 5.1 Write HTTP tests with `coretest` tokens: create a board with three preset columns; invalid name; list and read ordering; read with many tasks uses a fixed query count; rename; `PATCH` with an empty body leaves every field unchanged; delete hides the board and everything under it; add, rename (including clearing a preset's name and `400` on a custom column), move, and delete a column (`column_not_empty`, `last_column`, contiguous positions); `401`/`403` for each route
- [x] 5.2 Declare each route's policy (`gentasks.board.read` / `gentasks.board.manage`)
- [x] 5.3 Implement request types with validation, `BoardService` / column operations, handlers, and response types
- [x] 5.4 Run `make lint`, `make test`, `make build`
- [ ] 5.5 Developer sign-off, then open PR

## Phase 6 (branch: feat/gentasks-labels-api)

- [x] 6.1 Write HTTP tests: create with a palette color; unknown color; duplicate name ignoring case (`label_name_taken`); name reusable after delete; list ordered by name; `PATCH` color only leaves the name unchanged; `name: null` → `400`; delete detaches the label from tasks; `401`/`403`
- [x] 6.2 Declare route policies
- [x] 6.3 Implement request types, `LabelService`, handlers, and response types
- [x] 6.4 Run `make lint`, `make test`, `make build`
- [ ] 6.5 Developer sign-off, then open PR

## Phase 7 (branch: feat/gentasks-tasks-api)

- [x] 7.1 Write HTTP tests: minimal create lands last in the first column with urgency `none`; column or label from another board → `400`; unknown urgency → `400`; assignee valid, unknown (`400`), or core down (`503`) via `coretest`; `PATCH` single field leaves everything else unchanged; null rules for each field; `labelTokens` replaces the set; move within and across columns keeps positions contiguous; concurrent moves on one board; moving an archived task → `409 task_archived`; archive, archive twice, unarchive appends last; archived list ordering; delete active and archived; deleted board → `task_not_found`; `401`/`403` per route
- [x] 7.2 Declare route policies (`gentasks.task.write`, `gentasks.task.delete`, `gentasks.board.read`)
- [x] 7.3 Implement request types (with omitted and null tracking for `PATCH`), `TaskService` with the `AssigneeVerifier` interface, handlers, and response types, and wire `coreclient` in `app`
- [x] 7.4 Run `make lint`, `make test`, `make build`
- [ ] 7.5 Developer sign-off, then open PR

## Phase 8 (branch: feat/cli-module-provisioning)

- [x] 8.1 Write tests with the fake runner: missing `GENWORK_MODULE_SECRET` / `GENTASKS_DB_PASSWORD` are generated and existing values are kept; the provisioning script runs after Postgres starts, passes the password as a `psql` variable (never in arguments), and is idempotent; `genwork db`, `api`, and `up` all provision
- [x] 8.2 Implement the secret generation (reusing the first-run key filling) and the provisioning SQL file and runner
- [x] 8.3 Print a hint when `web/.env` still points `DEV_API_PROXY_TARGET` at core instead of the gateway
- [x] 8.4 Run `make lint`, `make test`, `make build`
- [ ] 8.5 Developer sign-off, then open PR

## Phase 9 (branch: feat/deploy-caddy-gateway)

- [x] 9.1 Write a gentasks test that asserts `deploy/Caddyfile` routes its manifest `apiPrefix`
- [x] 9.2 Add `deploy/Caddyfile` (internal paths blocked, core and gentasks routes, `404` fallback) and the `gateway` service with `GATEWAY_PORT`
- [x] 9.3 Add the `gentasks-migrate` and `gentasks` services (no host port), and the pinned compose network subnet used by `TRUSTED_PROXY_CIDRS`
- [x] 9.4 Update `deploy/.env.example` and `web/.env.example` (gateway target) and the CLI endpoint printout (gateway URL)
- [x] 9.5 Verify manually: `genwork up`, `/internal/*` → `404` through the gateway, capabilities list gentasks, `psql` as `gentasks` cannot read `core.users`
- [x] 9.6 Run `make lint`, `make test`, `make build`
- [ ] 9.7 Developer sign-off, then open PR

## Phase 10 (branch: feat/web-gentasks-logic)

- [x] 10.1 Add MSW handlers and fixtures in `web/src/test/` for capabilities, users, and every gentasks endpoint
- [x] 10.2 Write tests for `shared/domain/capabilities` (enabled, empty, failed request → all disabled) and `shared/datasource/capabilities` + `users`
- [x] 10.3 Write tests for gentasks `domain/`: urgency order and keys, column display name (preset vs custom), optimistic task and column move and rollback, filters keeping stored order, minimal `PATCH` diff, permission helpers, palette to CSS variable, API error code to message key
- [x] 10.4 Write tests for gentasks `datasource/`: request paths, methods, bodies, and error mapping for boards, columns, labels, and tasks
- [x] 10.5 Implement the code under test
- [x] 10.6 Run `make lint`, `make test`, `make build`
- [ ] 10.7 Developer sign-off, then open PR

## Phase 11 (branch: feat/web-gentasks-ui)

- [x] 11.1 Add `dialog`, `textarea`, and `alert-dialog` with `npx shadcn@latest add` (the existing `select` and `dropdown-menu` checkbox items cover the pickers), and `@dnd-kit/core` + `@dnd-kit/sortable`; review `package.json` and the lockfile (rule 77)
- [x] 11.2 Add the `--label-*` and urgency theme variables to `web/src/index.css` for both themes
- [x] 11.3 Add `CapabilitiesProvider`, the sidebar Tasks entry shown only when gentasks is enabled, and the "module unavailable" guard
- [x] 11.4 Implement `BoardsPage` and `BoardPage` with columns, cards, keyboard-accessible drag and drop, the task sheet, column and label management, archived view, and filters; register the `/tasks` and `/tasks/$boardToken` routes
- [x] 11.5 Add en and pt-BR translations for all gentasks copy, including preset column names, urgency levels, palette names, and error messages
- [ ] 11.6 Verify manually through the gateway: create a board, drag cards and columns, edit a single field, archive and restore, delete, labels, filters, read-only user, pt-BR
- [x] 11.7 Run `make lint`, `make test`, `make build`
- [ ] 11.8 Developer sign-off, then open PR
