## Context

State after the `gentasks-module` change:
- `deploy/docker-compose.yml` lists the gentasks services unconditionally, and the gateway `depends_on` gentasks.
- `deploy/Caddyfile` is committed with a hand-written gentasks route.
- The CLI provisions databases from a hard-coded `moduleDatabases` list.
- Core stores `modules.enabled`, sets it to `true` on first registration, and never changes it afterwards. Capabilities list only enabled modules.
- The CLI already runs core subcommands inside the core container (`core admin create`), can check that core is running (`ensureCoreIsRunning`), and sends psql scripts over stdin.
- The gateway runs Caddy with `admin off`, so its configuration cannot be hot-reloaded.

See `proposal.md` for the motivation.

## Goals / Non-Goals

**Goals:**
- One interaction for everything: a checklist where checked means installed.
- One canonical record of the installation's choice, written by the CLI only.
- Adding the next module needs only its folder, its `module.yaml`, two compose services under its profile, and its `.env.example` lines. No CLI code changes.
- Every install and uninstall step is idempotent, so rerunning finishes an interrupted change.

**Non-Goals:** see the proposal. At the design level there is also no daemon, no file watcher, and no automatic reconciliation.

## Decisions

### 1. `deploy/genwork.yaml` records the choice; core holds the applied state (rules 7, 56, 64)

```yaml
# Written by genwork. Choose modules with `genwork modules`.
modules:
  - gentasks
```

- **File missing:** the modules were never chosen, which triggers the first-run checklist.
- **Empty list:** the installation chose nothing.

Core's `modules.enabled` is the applied copy core needs at runtime to answer capabilities. Core cannot read the CLI's file, because they live on different hosts and in different containers. The CLI is the only writer of both:
- **Install:** add to the file first, then set enabled in core.
- **Uninstall:** disable in core first, then remove from the file last.

That way an interrupted run always leaves the module checked in the next checklist while its state is incomplete, and checking or unchecking it again finishes the job.

*Alternative considered:* core as the only owner. Rejected because `genwork up` must know which containers to start before core runs.

### 2. Catalog through `sdk/go/manifest` (rules 6, 51)

The CLI adds `github.com/JoaoBittencourt1/Genwork/sdk/go` with a `replace => ../sdk/go` directive, and parses `modules/*/module.yaml` with `manifest.ParseManifest`, the same code modules use to register. The CLI is built from the repository (`make install-cli`), so the `replace` resolves.

### 3. CLI layout (rules 4, 9)

```
cli/internal/modules/           pure logic: no docker, no cobra, no terminal
  catalog.go                    ModuleCatalog: LoadModuleCatalog(root) → modules + warnings
  installation.go               InstallationConfig: read/write deploy/genwork.yaml (atomic write)
  plan.go                       PlanSelection(catalog, installed, checked) → SelectionPlan or error
  naming.go                     ModuleConventions: profile, services, role/schema, env names from the id
  gateway.go                    RenderCaddyfile(installed manifests) → bytes; core routes embedded
cli/internal/commands/
  modules.go                    `genwork modules`: checklist → plan → summary → confirm → apply
  module_selector.go            ModuleSelector interface; huh implementation (checklist + confirmation)
  module_changes.go             installModule / uninstallModule steps over the runner
  provisioning.go               provision and deprovision any module, by convention
  stack.go                      first-run checklist, profiles of installed modules, Caddyfile rendering
```

`commands` depends on small interfaces for the runner, the selector, and terminal detection, so tests use fakes and never call Docker (rule 25).

### 4. Conventions derived from the module id

For an id like `gentasks` (dashes become underscores, then upper-cased):

| Fact | Value |
| --- | --- |
| compose profile | `gentasks` |
| services | `gentasks`, `gentasks-migrate` |
| database role and schema | `gentasks` |
| password / URL variables | `GENTASKS_DB_PASSWORD`, `GENTASKS_DATABASE_URL` |
| gateway upstream variable | `GENTASKS_UPSTREAM` |

The conventions live in one place (`naming.go`). Hosts and ports stay in `.env` (rule 1): the CLI only fills secrets and appends missing keys from `.env.example`, as it already does.

### 5. Selection plan

`PlanSelection` turns the checked ids into a `SelectionPlan` with these steps:

1. **Install set:** the closure of checked ids over `requires`. Each catalog version is checked against the required range with `Masterminds/semver`, which already comes in through the SDK, and cycles are detected with an in-progress set.
2. **Uninstall set:** installed modules that are not in the install set.
3. **Refusal:** the plan is refused if a module in the install set requires one in the uninstall set. Because of step 1 this can only happen for an explicitly unchecked module that another checked module requires, and the refusal names that module.
4. **Order:** installs go dependencies first. Uninstalls go dependents first.
5. **Summary:** the plan carries "added because X requires it" notes for the summary.

The plan is pure and fully unit-tested. The command layer only renders and applies it.

### 6. Applying the plan

| Install, per module | Uninstall, per module |
| --- | --- |
| 1. add to file | 1. `rm -s -f <id> <id>-migrate` |
| 2. ensure `.env` secrets | 2. `core module disable <id>` |
| 3. `up -d --wait postgres` + provision | 3. deprovision: `DROP SCHEMA IF EXISTS <id> CASCADE; DROP ROLE IF EXISTS <id>` over psql stdin |
| 4. render Caddyfile | 4. clear its two `.env` values |
| 5. `--profile <installed…> up -d --build <id>-migrate <id>` | 5. remove from file |
| 6. `core module enable <id>` | 6. render Caddyfile |
| 7. `restart gateway` | 7. `restart gateway` |

The gateway restart and the Caddyfile rendering run once after the whole plan, not once per module. `core module …` runs through `ensureCoreIsRunning` plus `exec`.

Uninstall is the only place in the platform where module data is physically deleted. It is the explicit, confirmed action rule 56 describes, and the documented exception to rule 22.

### 7. Confirmation

The summary always asks for confirmation, and the default answer is "no". When the plan uninstalls anything, the prompt lists each module and states that its data will be permanently deleted. *Alternative considered:* typing the module name to confirm. It can be added later if a plain confirm proves too easy to accept by accident.

### 8. Checklist with `huh`

`ModuleSelector` has two methods:
- `ChooseModules(options []ModuleOption) ([]string, error)`, backed by `huh`'s `MultiSelect`, with pre-selected options for installed modules. The space bar toggles and Enter confirms; these are `huh`'s defaults, and its key map allows changing them.
- `ConfirmPlan(summary string) (bool, error)`, backed by `huh`'s `Confirm`.

Each option is labelled `id  version  requires: …`, and installed modules missing from the catalog are labelled as missing.

- *Why `huh`:* it is maintained, has an accessible mode for screen readers, and needs no CGO.
- Terminal detection reuses `golang.org/x/term`, which the CLI already depends on.

### 9. First run in `up` and `api`

Before starting services, `up` and `api` load the installation config. If the file is missing:
- **With a terminal:** they run the checklist with nothing checked, then apply the plan (installs only) and the file is saved even when empty.
- **Without a terminal:** they save an empty choice and print the `genwork modules` hint.

They then start the stack with one `--profile` per installed module. `db` does not ask, because it starts only Postgres. `down` passes `--profile "*"`.

### 10. Generated Caddyfile

The template is embedded in the CLI and holds the global options, the `/internal/*` block, and the core routes. One `handle <apiPrefix>/* { reverse_proxy {$<ID>_UPSTREAM} }` block is added per installed module, sorted by id. The file is written atomically, and `deploy/Caddyfile` and `deploy/genwork.yaml` go into `.gitignore`.

The gateway is restarted rather than reloaded, because `admin off` keeps Caddy's admin API closed. A restart takes about a second and is acceptable locally.

### 11. Compose profiles and consistency test

The gentasks services get `profiles: ["gentasks"]`, and the gateway's `depends_on: gentasks` is removed, because compose refuses to start a service that depends on one in an inactive profile. A CLI test parses `deploy/docker-compose.yml` and checks every catalog module against decision 4. It also checks that no service outside a module's profile depends on a module service. That test replaces the gentasks test that read the committed Caddyfile.

### 12. Core subcommands (rules 52, 82)

`core module enable|disable <id>` live in `core/internal/command/module.go`. They call `modules.ModuleService.SetEnabled(ctx, name, enabled) (found bool, err error)`, backed by `ModuleRepository.SetEnabled`, which is one `UPDATE … WHERE name = $1` that reports whether a row matched.
- An unknown id is not an error: the module registers itself enabled when it starts.
- Registration keeps not touching `enabled`, which is why reinstalling calls `enable` explicitly.
- Retired permissions and role grants are untouched by uninstall, so a reinstalled module gets back the grants it had.

## Risks / Trade-offs

- [Unchecking a box deletes data] → A summary and an explicit confirmation that names the data loss, defaulting to "no", sit between the checklist and any deletion.
- [The file and core disagree after an interrupted run] → The ordering in decision 1 keeps the module checked while incomplete, and every step is idempotent, so rerunning converges.
- [The gateway restart drops in-flight requests for about a second] → It is local only and happens after a change of modules.
- [Existing checkouts see the first-run checklist on the next `genwork up`] → Checking gentasks reuses its schema and data, because provisioning is idempotent and migrations are versioned.
- [`huh` widens the CLI's dependency tree] → It is reviewed under rule 77, is CLI-only, and never reaches the product images.

## Migration Plan

1. The core phase ships first and changes no behavior on its own.
2. The CLI phases ship next.
3. The deploy phase puts gentasks behind its profile and stops committing the Caddyfile.
4. On the next `genwork up`, developers check gentasks in the first-run checklist. Their data is reused.

Rollback: revert the deploy phase to get the unconditional services back. The core subcommands are harmless when unused.
