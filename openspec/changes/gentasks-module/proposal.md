## Why

Genwork has a core but no modules yet, so the platform cannot do the work it exists for. Gentasks is the first module: a task board where people organize work in columns and move task cards through them. Because it is the first module, it also has to build the extension points every later module needs. Today core has no module registry, no capabilities endpoint, no way for a module to declare permissions or validate tokens, no user directory, and no gateway.

## What Changes

- **Core — module registry (rules 51, 52, 58, 60):** modules register on startup with the facts from their `module.yaml` (id, version, API prefix, permissions). Core stores the module and its permissions and grants each permission to the default roles the manifest names, the first time it sees that permission. Registration runs on an internal route that the gateway does not expose, and it is authenticated with a shared module secret.
- **Core — capabilities:** `GET /api/core/v1/capabilities` lists the enabled modules with their versions and API prefixes.
- **Core — user directory:** `GET /api/core/v1/users` (new permission `core.user.read`) lets clients pick people, and `GET /api/core/v1/users/{token}` lets modules confirm that a user token exists.
- **Core — access token:** the access token gains a `permissions` claim, so modules can authorize requests without calling core (rule 57).
- **New `sdk/go/`:** parses and validates `module.yaml`, registers a module with core, verifies access tokens against core's JWKS, and provides permission middleware and a core client. `coretest` stubs core with recorded contract fixtures (rule 62).
- **New module `modules/gentasks/`** (its own container, Postgres schema, database role, and release):
  - **Boards:** create, list, rename, delete. Each new board starts with three columns: Backlog, To-do, Done.
  - **Columns:** create, rename, reorder, and delete. A column that still holds tasks cannot be deleted, and neither can a board's last column.
  - **Labels:** created per board with a name and a color from a fixed palette. They can be renamed, recolored, and deleted.
  - **Tasks (cards):** title, description, optional assignee (a core user), labels, and urgency. Urgency is one of five values: `none`, `low`, `medium`, `high`, `urgent`. Tasks can be moved within a column and between columns, archived and restored, and deleted. Delete is a soft delete (rule 22).
- **Gateway (deploy):** Caddy becomes the single public entrypoint (rule 57). It routes `/api/core/...` and `/.well-known/...` to core and `/api/gentasks/v1/...` to the gentasks container, and it blocks core's internal routes. The Vite dev proxy now points at the gateway instead of core. **BREAKING (local setup):** `deploy/.env` and `web/.env` gain new required variables.
- **CLI:** `genwork up`/`api` start the gateway and gentasks. They create the `gentasks` database role and schema idempotently and generate the module secret and the role password into `.env` on first run.
- **Web:** a shared capabilities client, plus a new `web/src/app/gentasks/` with a boards list page and a board page. The board page supports dragging cards between columns, reordering columns, creating and editing tasks, managing labels, viewing archived tasks, and filtering by urgency, label, and assignee. A sidebar entry appears only while gentasks is enabled. The UI is in pt-BR and en.

## Components touched

core, sdk, modules/gentasks, cli, deploy, web. No native clients.

Spanning core and a module is justified (rule 63): gentasks cannot run without the generic extension points above, and no other module exists yet that could drive their design. Every phase in `tasks.md` touches one component only.

### `module.yaml` facts (gentasks)

| Fact | Value |
| --- | --- |
| id | `gentasks` |
| version | `0.1.0` |
| core API range | `>=1.0.0 <2.0.0` (core API `v1`) |
| requires | none |
| API prefix | `/api/gentasks/v1` |
| permissions | `gentasks.board.read`, `gentasks.board.manage` (boards, columns, labels), `gentasks.task.write` (create, edit, move, archive), `gentasks.task.delete`; all granted to `admin` and `member` by default |
| events published | none |
| events consumed | none |

## Capabilities

### New Capabilities

- `module-registry`: how a module registers with core, how its permissions reach roles, and the capabilities endpoint.
- `module-authentication`: the access token's permissions claim and how modules validate tokens and authorize requests.
- `user-directory`: listing users and looking one up by token.
- `api-gateway`: public routing to core and modules, and blocking internal routes.
- `module-database-provisioning`: the CLI creating a module's isolated database role and schema and generating its secrets.
- `gentasks-boards`: boards and their columns, including the default columns, ordering, and deletion rules.
- `gentasks-labels`: per-board labels with palette colors.
- `gentasks-tasks`: task cards — fields, urgency, assignee, labels, moving, archiving, and deletion.
- `web-module-capabilities`: the web app showing only enabled modules.
- `gentasks-web-ui`: the gentasks screens in the web app.

### Modified Capabilities

_(none — `openspec/specs/` is still empty because `core-auth` has not been archived. The new token claim is specified under `module-authentication`.)_

## Non-goals

- Enabling, disabling, and uninstalling modules from the CLI, the full installer (`genwork install`), and pulling images (rule 64). For now gentasks is always part of the local stack.
- Module health polling. Capabilities report registered, enabled modules, not live health.
- An event bus and module events. Gentasks publishes and consumes nothing, and the open decision stays open.
- Serving the built SPA and TLS from Caddy in production. That belongs to the installer change.
- Linking boards to core projects or workspaces, which do not exist yet.
- Comments, attachments, subtasks, due dates, estimates, activity history, notifications, and real-time updates between browsers.
- Custom label colors outside the palette, and per-board permissions. Permissions are instance-wide through roles.
- Native clients.

## Impact

- **Code:** new in core: `core/internal/{modules,capabilities}`, the `users` handler extended, and the token claims extended. New `sdk/go/`, `modules/gentasks/`, and `deploy/Caddyfile`. CLI stack commands. New in web: `web/src/shared/{datasource,domain}/capabilities.ts` and `web/src/app/gentasks/`.
- **APIs:** `GET /api/core/v1/capabilities`, `GET /api/core/v1/users`, `GET /api/core/v1/users/{token}`, internal `PUT /internal/v1/modules/{id}`, and everything under `/api/gentasks/v1/`.
- **Database:** core migrations for `modules`, a module owner on `permissions`, and the `core.user.read` seed. A new `gentasks` schema with `boards`, `columns`, `labels`, `tasks`, and `task_labels`, owned by a dedicated `gentasks` role.
- **Dependencies:** Go: `github.com/Masterminds/semver/v3` in core and `sdk/go`, plus `gopkg.in/yaml.v3` and `golang-jwt` in `sdk/go`. Web: `@dnd-kit/core`, `@dnd-kit/sortable`, and shadcn components (dialog, textarea, alert-dialog). Deploy: the `caddy` image.
- **Configuration:** `GENWORK_MODULE_SECRET`, `GATEWAY_PORT`, `CORE_UPSTREAM`, `GENTASKS_UPSTREAM`, `GENTASKS_DB_PASSWORD`, `GENTASKS_DATABASE_URL`, `CORE_INTERNAL_URL`, and `TRUSTED_PROXY_CIDRS` set to the compose network. `DEV_API_PROXY_TARGET` now points at the gateway.
- **Build/CI:** new Makefile targets `lint-sdk`, `test-sdk`, `lint-gentasks`, `test-gentasks`, and `build-gentasks`, reached through `make lint`/`test`/`build` (rule 75).
