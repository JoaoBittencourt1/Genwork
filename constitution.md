# Constitution

Rules that MUST be followed in this repository. They are non-negotiable.

Genwork is a self-hosted platform built from **modules**: a small core plus optional modules (kanban, infra insights, automation, …) that each installation picks through the installer CLI. See the Modules section — it shapes every other rule.

This is a monorepo. Components and stacks:

| Path | Stack |
| --- | --- |
| `core/` | Go service: auth, users, workspaces & teams, roles & permissions, projects, module registry, settings |
| `modules/<name>/` | Go service per module, each its own container and release |
| `sdk/go/` | Shared Go library modules use to talk to core (registration, token validation, events) |
| `cli/` | Go installer CLI for self-hosted setups |
| `web/` | SPA: Vite + React (TypeScript), served as static files |
| `clients/macos/` | Swift / SwiftUI |
| `clients/windows/` | C# / .NET (UI framework TBD) |
| `clients/linux/` | Rust / GTK 4 |

Rules are numbered and the numbers are stable: never renumber, append new rules at the end of their section with the next free number, and reference them as "rule N" in specs and PRs.

## Configuration & secrets

1. **Never hardcode URLs, hosts, ports, keys, or credentials in source code.** They come from environment variables. If a required variable is missing, fail explicitly at startup — no silent fallbacks like `?? "http://localhost:..."`.
2. **Never pin environment-specific values in `docker-compose.yml`'s `environment:` block.** It overrides `env_file`, so the same compose file silently diverges between local and production (e.g. a hardcoded `SMTP_HOST: mailpit` would send all production e-mail to a local mail catcher). Values flow from `.env`; placeholders live in `.env.example`.
3. **Never commit secrets** (`.env`, private keys, cloud credentials). Document placeholders in `.env.example` only.

## Architecture & code organization

4. **Organize business code by feature, not by type.** In every Go service (core and each module), each feature lives in `internal/<feature>/`; entrypoints live in `cmd/<binary>/`. Code shared by two or more features moves to a shared package instead of being duplicated.
5. **Name every file and type for exactly what it is**, with its architectural role (`TaskHandler`, `TaskService`, `TaskRepository`, `CreateTaskRequest`, `TaskResponse`). No generic names like `Manager`, `Util`, `Helper`, `Data`, or `Misc`.
6. **Search before adding.** Before creating a type, package, migration, component, or config entry, look for existing code that can be reused or extended. Reuse or refactor before duplicating.
7. **Single source of truth.** Every fact, state slice, label, mapping, or identifier has exactly one canonical owner; everything else derives from it. If two sources already exist, consolidate them in the same change — never add a third.
8. **Keep handlers/controllers thin** — parse the request, delegate, write the response. Business logic lives in services.
9. **Keep framework and transport details out of business rules.** HTTP types, SQL drivers, UI frameworks, and storage APIs must not leak into domain logic. Depend on small interfaces at the boundary; wire concrete implementations at the composition root (`cmd/`, app entrypoints).
10. **Everything that is not core is a module** — see the Modules section (rules 49–64).
65. **The web app is a static SPA** (Vite + React), served by the gateway — no Node process runs in production. It calls the API on the same origin through relative `/api/...` paths, so no API URL is baked in at build time and one build works for every installation.
72. **The web app is organized by module under `web/src/app/<module>/`**, each with:
    - `datasource/` — HTTP calls through the shared client; no React.
    - `domain/` — types and pure rules (permissions, status transitions, validations); no React and no HTTP — when a rule needs data, it receives a gateway as a parameter.
    - `components/` — the module's React pieces, including its context, providers, and query hooks.
    - `<Name>Page.tsx` files at the module root — one per route.

    Cross-module code lives in `web/src/shared/` with the same split (`datasource/`, `domain/`, `hooks/`, `components/`), plus `ui/` (rule 73), `lib/`, and `i18n/`. Root wiring (`App.tsx`, `router.tsx`, `main.tsx`) stays in `web/src/`.
73. **The web UI is built from shadcn/ui components in `web/src/shared/ui/`**, added with `npx shadcn@latest add <component>` and never re-implemented by hand. Screens compose those components; reusable compositions (layouts, alerts with icons) live in `web/src/shared/components/`. Generated components may be adjusted in place, but never copied into a module. Before running `shadcn add`, every alias in `web/components.json` must resolve to an existing file — in particular `cn` in `web/src/shared/lib/utils.ts`; otherwise the CLI resolves the import to an unrelated npm package with the same name and installs it.

## Modules

49. **Core is minimal; everything else is a module.** Core owns only: auth, users, workspaces & teams, roles & permissions, projects, the module registry, and settings. Kanban, infra insights, automation, and any future feature are modules. When in doubt, it is a module. **Core must run and pass its tests with zero modules installed.**
50. **One module = one container.** Each module lives in `modules/<name>/` with its own `go.mod`, `cmd/`, `internal/`, `migrations/`, `Dockerfile`, and `module.yaml`; it is built into its own image and released with its own semantic version, independent of core and of other modules.
51. **`module.yaml` is the module's single source of truth** (rule 7): id, version, compatible core API range, required modules with version ranges, permissions it defines, events it publishes and consumes, and its API prefix. Nothing else may restate these facts.
52. **Core never contains module-specific code.** No `if module == "kanban"` in core. Core exposes generic extension points (registry, permissions, events, settings, capabilities) and modules plug into them.
53. **Each module owns its data in isolation.** One Postgres schema per module (named after the module), accessed through a dedicated database role that cannot read any other schema. No cross-schema foreign keys, joins, or queries. Core entities (user, workspace, project) are referenced by their opaque token (rule 14).
54. **Modules talk to each other only through public contracts**: the other module's versioned HTTP API or published events. Never through its database, never by importing its Go code. Code shared by several modules lives in `sdk/go/`.
55. **Module dependencies are declared, acyclic, and enforced by the installer.** A module may require another only if `module.yaml` lists it. Installing or enabling a module enables its required modules; disabling a module is refused while an enabled module requires it. An *optional* integration ("works better if X is present") is not a dependency: it must degrade gracefully — hide the feature, never error — when X is not enabled.
56. **Disable is not uninstall.** Disabling stops the module's container and hides it from clients; its data stays intact and re-enabling resumes where it left off. Uninstall is a separate, explicit action that confirms data removal. Enabling or disabling a module never requires a code or config change in core or in any other module.
57. **One public entrypoint.** A gateway routes `/api/core/...` to core and `/api/<module>/v<N>/...` to each module; module containers are never exposed directly. Core issues access tokens; modules validate them with core's public keys and never store credentials.
58. **Clients follow the enabled set at runtime.** Modules register with core on startup (manifest + health). Core publishes the enabled modules and their versions through a capabilities endpoint. The web app and desktop clients ship UI for every module, read capabilities, and show only enabled modules — no client code path may assume a module is enabled.
59. **Module code is isolated in every component.** Each client keeps a module's UI in a dedicated folder (e.g. `web/src/app/<name>/`, rule 72), and one module's UI never imports another's. Shared UI belongs to the client's shared design system.
60. **Permissions are namespaced `<module>.<resource>.<action>`** (e.g. `kanban.issue.create`), declared in `module.yaml`, and stored and assigned to roles by core.
61. **Public contracts are versioned.** A breaking change to a module's API or events — or to a core contract modules consume — ships as a new major version (`/v2`, new event version) while the previous one keeps working through a deprecation window.
62. **Modules are tested in isolation.** A module's suite runs against a stubbed core and stubbed dependency APIs built from recorded contract fixtures — no other real module running. CI runs: core alone; each module with only its declared dependencies; all modules together.
63. **Every OpenSpec change states which components it touches** (core, which modules, which clients). A change spanning core and a module, or several modules, justifies why, and its phases are split per component.
64. **The installer CLI is the supported way to choose modules.** `cli/` resolves dependencies, writes the deployment config and `.env` (rules 1–3), pulls the selected images, runs each module's migrations, and enables or disables modules.

### Open decisions — do not assume

These are not decided yet. Do not pick one silently in code or specs; raise it with the developer.

- Gateway technology (e.g. Caddy, Traefik, or a Go gateway inside core).
- Event bus technology (e.g. Postgres outbox + `LISTEN/NOTIFY`, NATS).
- Windows client UI framework.

## API design

11. **Validate input at the boundary**, on dedicated request types — not ad hoc inside business logic.
12. **Never bind a request body directly to a persistence model.** Decode into a request type and map explicitly (guards against mass assignment).
13. **Shape responses with explicit response types** — never serialize persistence models to clients.
14. **Every table has two identifiers: `id` and `token`.** `id` is the internal auto-generated integer primary key, used only for foreign keys and joins inside the owning service. `token` is the public identifier: opaque, unique, immutable, URL-safe, and generated by a cryptographically secure random generator with at least 128 bits of entropy. APIs, events, URLs, and references across services (rule 53) use only `token`; an `id` never leaves its service.
15. **Represent fixed sets of values as typed enums/constants**, not loose strings or ints.
16. **Prefix REST routes with `/api`**, followed by the owning component (rule 57).
17. **Partial update is `PATCH` and must distinguish "omitted" from "explicit null".** Omitted field → stored value unchanged. Present non-null → persist. Present `null` → clear if nullable, HTTP 400 if not. `PUT` is a full replace. Every `PATCH` endpoint has a test proving that a single-field body leaves every other stored field unchanged.
18. **Authorization is part of the feature, not a follow-up.** Every new endpoint declares its authorization rule when it is added; nothing is publicly reachable by omission. Resource-ownership checks live in one central place, not in per-feature copies.

## Security

66. **Passwords are hashed with Argon2id** — never encrypted or stored reversibly. Each hash has its own random salt and uses parameters at or above the current OWASP recommendation (baseline: 64 MiB memory, 3 iterations, parallelism 2). The stored hash encodes its parameters; when a login succeeds against outdated parameters, the password is rehashed transparently. Passwords are 8 to 128 characters (the cap bounds hashing cost), with no composition rules.
67. **Credentials and bearer secrets are never stored or logged in clear.** Refresh tokens, invite tokens, and any other bearer secret are stored only as a hash (SHA-256 suffices for high-entropy random values) and compared in constant time. Passwords, tokens, and `Authorization` headers never appear in logs, error messages, or traces.
68. **Authentication responses never reveal which part failed.** An unknown e-mail and a wrong password return the same error in comparable time (hash a dummy password when the account does not exist). Login and sign-up endpoints are rate limited per client address and per account, using the address the server resolves — never a client-controlled header such as the first `X-Forwarded-For` value.
69. **Access tokens are short-lived; refresh tokens rotate.** Access token: signed JWT, at most 15 minutes. Refresh token: opaque, bound to one session, single-use — each refresh issues a new one, and presenting an already-used one revokes the whole session. The web app keeps the refresh token in an `httpOnly; Secure; SameSite=Strict` cookie and the access token only in memory; native clients keep tokens in the OS secure store (Keychain, Windows Credential Manager, Secret Service).

## Persistence

19. **All schema changes go through versioned migrations** in the owning service's `migrations/` directory (`core/migrations/`, `modules/<name>/migrations/`), written as plain SQL for goose and embedded in the service binary. Never alter the database manually; never recreate existing tables.
20. **Never edit a migration that has already been applied** — including whitespace or comment-only edits, which change its checksum. Add a new migration instead. Formatters and comment-stripping tools must exclude the migrations directory.
21. **Avoid N+1 queries** when returning related data: join or batch — never query in a loop over results.
22. **Use soft delete for removable domain records** (`deleted_at` nullable timestamp). Unique constraints on soft-deletable tables use partial indexes `WHERE deleted_at IS NULL`. Physical deletes are reserved for test cleanup, or for an explicitly documented exception in the feature's spec.

## Testing

23. **Every feature or relevant change ships with tests** covering its main behavior, in the same change set.
24. **Write tests before the code** they cover (red → green → refactor).
25. **No test may call a real external API — ever.** The whole suite passes with no network and no credentials. Stub outbound clients and feed them recorded fixtures committed to the repo. Enforce it through configuration: the test environment points every outbound base URL at an unroutable address and every key at an obviously fake value, so an accidental real call fails instead of silently succeeding and billing.
26. **Fixtures and test doubles live in dedicated files**, not as large inline object graphs in test bodies. Test bodies stay thin: arrange via fixtures, act, assert.
27. **Do not test seeders** (dev/demo data only) and keep them out of coverage gates. Logic a seeder needs belongs in a regular, tested unit.
70. **Every component keeps at least 75% coverage**, measured over all of its own source files — statements for Go (`go test -coverpkg=./...`) and lines and statements for the web app (Vitest, v8) over its logic folders only (`domain/` and `datasource/`, rule 74). The minimum lives once, in `MIN_COVERAGE` in the Makefile; `make test` and CI fail below it. Only test-support code and generated files may be excluded. Never lower the minimum or add exclusions to make a change pass — write the missing tests.
71. **Tests live next to the code they test.** In Go, `*_test.go` files sit in the same package directory, which is what lets them reach unexported code and count toward that package's coverage; cross-package HTTP integration tests live in `core/internal/app`, and reusable test doubles live in dedicated `*test` packages (e.g. `mailtest`, `dbtest`). In the web app, `*.test.ts(x)` sits beside the module it tests, and shared helpers, fixtures, and API mocks live in `web/src/test/`.
74. **Web tests cover logic, not markup.** Only `domain/` and `datasource/` code is unit-tested — rules, mappings, request shapes, and error handling, with the API stubbed by MSW (rule 25). Components, pages, and `shared/ui` are not unit-tested and do not count toward coverage; keep them thin by moving every decision they make into `domain/`.
81. **The brand palette is black and green.** Dark is the default theme: near-black backgrounds with the green of the logo (`--primary`, `--ring`) as the single accent. Colors come only from the theme variables in `web/src/index.css`; screens never hardcode color values. The master logo lives in `assets/logo.png`; the web app serves small derivatives from `web/public/` (favicon and touch icon).

## Code quality

28. **Small functions, single purpose.** If you describe it with "and then… and after that…", split it.
29. **Names say exactly what things do.** Functions start with a verb (`validateEmail`, not `handleData`); variables express intent (`taskIndex`, not `i` outside trivial loops). Avoid `process()`, `handle()`, `calculate()`.
30. **Use consistent vocabulary.** Pick `find` *or* `get` for the same idea and stick to it.
31. **Comment only when strictly necessary.** The default is no comment. A comment is justified only for a non-obvious external constraint, a business rule names cannot express, a workaround for an external bug, or a `TODO`/`FIXME` describing the debt and its fix. Explain the *why*, never the *what*. Longer rationale belongs in `openspec/` or the PR description.
32. **Explicit error handling.** Return or throw meaningful errors; never swallow them; don't mix business logic with error-handling noise.
33. **Delete dead code.** Leave code cleaner than you found it; refactoring is first-class work. When refactoring, prefer clarity over conciseness.
34. **Do not use deprecated APIs**; fix deprecation warnings in the code you touch.

## Language

35. **Everything in source is English**: file names, identifiers, comments, log messages, error messages, test names. Specs in `openspec/` are written in English too.
36. **User-facing copy is never hardcoded.** It goes through each component's localization mechanism (message keys + translation files); pt-BR is a first-class locale.
37. **Commit messages and PR descriptions are written in pt-BR.**

## Workflow

38. **Spec before code.** Every feature starts as an OpenSpec change (`/opsx:propose`) before implementation.
39. **Branches and commits** follow `tipo(N-<issue>)/short-slug-in-english` and `tipo(N-<issue>): mensagem em pt-BR`, where `tipo` is `feat`, `fix`, `chore`, `refactor`, `test`, `docs`, `ci`, etc. PRs link their GitHub issue.
40. **Run the component's lint at the end of every task and before every commit and push.** Never commit or push on a red lint, test, or build, and never bypass hooks with `--no-verify`.
41. **Never commit or push without developer sign-off.** After lint and tests pass, stop and let the developer verify the behavior manually. Automated tests alone are not sufficient.
75. **Local gates are the CI gates.** Every check CI runs is reachable through `make lint`, `make test`, and `make build`, and CI invokes only those targets — never a check that exists only in the workflow file (this includes `npm audit`, which runs inside `make lint-web`). Run all three before every push.
76. **Rename case-only changes with `git mv`.** macOS and Windows file systems ignore letter case, so deleting `Button.tsx` and creating `button.tsx` keeps the old name in git while everything works locally; CI runs on case-sensitive Linux and the import fails there. Use `git mv old new` (or `git mv -f` when only the case differs), and keep file names exactly as their generators emit them (shadcn components are lowercase).
80. **Read the whole diff before every commit**, including files changed by scripts and generators. A scripted edit that rewrites a file must be checked against the base branch for anything removed unintentionally — once, a rule insertion silently dropped every section after it.

## Dependencies

77. **Review every dependency change before committing.** After any generator or package command (`npx shadcn add`, `npm install`, `go get`, `go mod tidy`), read the diff of `package.json`, the lockfile, or `go.mod` and justify each added package. Remove anything added by mistake or left unused.
78. **Developer CLIs are not project dependencies.** Run code generators and scaffolding tools on demand (`npx <tool>@latest`, `go run`) instead of installing them; their transitive dependencies widen the audit surface without reaching the product. When a tool ships an asset the product needs (e.g. a stylesheet), copy the asset into the repository with a header naming its source and version, and refresh it when upgrading.
79. **No known high-severity vulnerabilities.** `npm audit --audit-level=high` must pass for the web app (part of `make lint-web`). Fix by upgrading or removing the dependency; overriding a transitive version is a last resort and needs a comment explaining why.

## Phased delivery

42. **Split feature work into numbered phases** in the change's `tasks.md`; each phase ships on its own branch with one PR.
43. **Before generating code, produce a dependency graph, layer assignment, and a PR plan table** (`| Phase | Contents | Depends on | Parallel with |`). Do not implement until the plan is approved.
44. **Dependency first, one layer per PR.** Artifacts with zero internal dependencies form the first PR. Never mix layers in a PR, and never ship an interface and its implementation in the same PR. If an upper layer ships first, use stubs until the dependency merges.
45. **PRs in the same layer may be reviewed in parallel** only when they do not depend on each other.
46. **Cap each PR at ~600 productive lines and 10 new files**; subdivide before implementing if exceeded.
47. **Go service phases (core and modules) implement in this order:** test → migration → model → repository → authorization → request type → service → handler → response type.
48. **Every phase includes its own tests** covering only the code delivered in that phase. CI must pass after each phase.

## Samples

`tasks.md`:

```markdown
## Phase 1 (branch: feat(N-12)/task-crud-foundation)
- [ ] 1.1 Write tests for phase 1
- [ ] 1.2 Implement …
- [ ] 1.3 Run lint and tests
- [ ] 1.4 Open PR

## Phase 2 (branch: feat(N-12)/task-crud-api)
- [ ] 2.1 Write tests for phase 2
- [ ] 2.2 Implement …
- [ ] 2.3 Run lint and tests
- [ ] 2.4 Open PR
```

Phased delivery table:

```markdown
| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | Migration + model + repository | — | — |
| 2 | Service + handler + DTOs | Phase 1 merged | — |
```
