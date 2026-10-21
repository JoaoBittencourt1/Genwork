# Constitution

Rules that MUST be followed in this repository. They are non-negotiable.

This is a monorepo. Components and stacks:

| Path | Stack |
| --- | --- |
| `server/` | Go (API + worker), PostgreSQL |
| `web/` | Next.js (TypeScript) |
| `clients/macos/` | Swift / SwiftUI |
| `clients/windows/` | C# / .NET (UI framework TBD) |
| `clients/linux/` | Rust / GTK 4 |

Rules are numbered and the numbers are stable: never renumber, append new rules at the end of their section with the next free number, and reference them as "rule N" in specs and PRs.

## Configuration & secrets

1. **Never hardcode URLs, hosts, ports, keys, or credentials in source code.** They come from environment variables. If a required variable is missing, fail explicitly at startup — no silent fallbacks like `?? "http://localhost:..."`.
2. **Never pin environment-specific values in `docker-compose.yml`'s `environment:` block.** It overrides `env_file`, so the same compose file silently diverges between local and production (in a previous project a hardcoded `MAIL_HOST: mailpit` sent all production e-mail to the local Mailpit). Values flow from `.env`; placeholders live in `.env.example`.
3. **Never commit secrets** (`.env`, private keys, cloud credentials). Document placeholders in `.env.example` only.

## Architecture & code organization

4. **Organize business code by feature, not by type.** On the server, each feature lives in `server/internal/<feature>/`; entrypoints live in `server/cmd/<binary>/`. Code shared by two or more features moves to a shared package instead of being duplicated.
5. **Name every file and type for exactly what it is**, with its architectural role (`TaskHandler`, `TaskService`, `TaskRepository`, `CreateTaskRequest`, `TaskResponse`). No generic names like `Manager`, `Util`, `Helper`, `Data`, or `Misc`.
6. **Search before adding.** Before creating a type, package, migration, component, or config entry, look for existing code that can be reused or extended. Reuse or refactor before duplicating.
7. **Single source of truth.** Every fact, state slice, label, mapping, or identifier has exactly one canonical owner; everything else derives from it. If two sources already exist, consolidate them in the same change — never add a third.
8. **Keep handlers/controllers thin** — parse the request, delegate, write the response. Business logic lives in services.
9. **Keep framework and transport details out of business rules.** HTTP types, SQL drivers, UI frameworks, and storage APIs must not leak into domain logic. Depend on small interfaces at the boundary; wire concrete implementations at the composition root (`cmd/`, app entrypoints).
10. **Optional features ship as modules** under `server/internal/modules` and must be disableable without breaking the core.

## API design

11. **Validate input at the boundary**, on dedicated request types — not ad hoc inside business logic.
12. **Never bind a request body directly to a persistence model.** Decode into a request type and map explicitly (guards against mass assignment).
13. **Shape responses with explicit response types** — never serialize persistence models to clients.
14. **Expose public resource identifiers as opaque tokens**, never internal sequential IDs.
15. **Represent fixed sets of values as typed enums/constants**, not loose strings or ints.
16. **Prefix REST routes with `/api`.**
17. **Partial update is `PATCH` and must distinguish "omitted" from "explicit null".** Omitted field → stored value unchanged. Present non-null → persist. Present `null` → clear if nullable, HTTP 400 if not. `PUT` is a full replace. Every `PATCH` endpoint has a test proving that a single-field body leaves every other stored field unchanged.
18. **Authorization is part of the feature, not a follow-up.** Every new endpoint declares its authorization rule when it is added; nothing is publicly reachable by omission. Resource-ownership checks live in one central place, not in per-feature copies.

## Persistence

19. **All schema changes go through versioned migrations** in `server/migrations/`. Never alter the database manually; never recreate existing tables.
20. **Never edit a migration that has already been applied** — including whitespace or comment-only edits, which change its checksum. Add a new migration instead. Formatters and comment-stripping tools must exclude the migrations directory.
21. **Avoid N+1 queries** when returning related data: join or batch — never query in a loop over results.
22. **Use soft delete for removable domain records** (`deleted_at` nullable timestamp). Unique constraints on soft-deletable tables use partial indexes `WHERE deleted_at IS NULL`. Physical deletes are reserved for test cleanup, or for an explicitly documented exception in the feature's spec.

## Testing

23. **Every feature or relevant change ships with tests** covering its main behavior, in the same change set.
24. **Write tests before the code** they cover (red → green → refactor).
25. **No test may call a real external API — ever.** The whole suite passes with no network and no credentials. Stub outbound clients and feed them recorded fixtures committed to the repo. Enforce it through configuration: the test environment points every outbound base URL at an unroutable address and every key at an obviously fake value, so an accidental real call fails instead of silently succeeding and billing.
26. **Fixtures and test doubles live in dedicated files**, not as large inline object graphs in test bodies. Test bodies stay thin: arrange via fixtures, act, assert.
27. **Do not test seeders** (dev/demo data only) and keep them out of coverage gates. Logic a seeder needs belongs in a regular, tested unit.

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

## Phased delivery

42. **Split feature work into numbered phases** in the change's `tasks.md`; each phase ships on its own branch with one PR.
43. **Before generating code, produce a dependency graph, layer assignment, and a PR plan table** (`| Phase | Contents | Depends on | Parallel with |`). Do not implement until the plan is approved.
44. **Dependency first, one layer per PR.** Artifacts with zero internal dependencies form the first PR. Never mix layers in a PR, and never ship an interface and its implementation in the same PR. If an upper layer ships first, use stubs until the dependency merges.
45. **PRs in the same layer may be reviewed in parallel** only when they do not depend on each other.
46. **Cap each PR at ~600 productive lines and 10 new files**; subdivide before implementing if exceeded.
47. **Server phases implement in this order:** test → migration → model → repository → authorization → request type → service → handler → response type.
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
