## Why

Every module is hard-wired into the local stack today: `genwork up` always starts gentasks, and an installation cannot choose its modules, add one later, or remove one. Rule 64 makes the installer CLI the supported way to choose modules, and rules 55 and 56 define how dependencies and removal behave. This change lets people pick modules through a checklist, both on the first `genwork up` and at any time afterwards, without touching code or configuration in core or in other modules.

## What Changes

- **First run:** the first `genwork up` (or `genwork api`) on an installation that has not chosen its modules yet shows a checklist of every module in the catalog, all unchecked. Space toggles a module and Enter confirms. The checked modules are installed and the choice is saved, so later runs do not ask again. Confirming with nothing checked is a valid choice.
- **`genwork modules`:** shows the same checklist, with installed modules already checked. Checking a module installs it. Unchecking an installed module uninstalls it. Before applying, the CLI shows a summary of what will be installed and what will be uninstalled. Uninstalling needs an explicit confirmation that names the data to be deleted (rule 56). This works whether or not the stack is running.
- **Dependencies (rule 55):**
  - Checking a module also installs the modules it requires, and the summary shows why each one was added.
  - The CLI refuses to uninstall a module that a module staying installed requires, and names that module.
  - A selection with an unknown module, a version range the catalog cannot satisfy, or a dependency cycle is refused before anything changes.
- **Installing a module:**
  1. generates its secrets
  2. provisions its database role and schema
  3. runs its migrations
  4. starts its containers
  5. routes it through the gateway

  The module registers itself with core, so core is not restarted.
- **Uninstalling a module:**
  1. stops and removes its containers
  2. tells core to stop publishing it, so clients hide it
  3. drops its database schema and role, deleting all its data
  4. clears its secrets
  5. removes its gateway route

  This is the documented exception to soft delete for module data (rules 22, 56).
- **Core:** new subcommands `core module enable <id>` and `core module disable <id>` set the state that the capabilities endpoint publishes. The CLI runs them inside the core container, the same way it runs `core admin create`. Uninstall uses `disable`. Install uses `enable`, so a module that was uninstalled and later reinstalled is published again.
- **Installation config:** a new `deploy/genwork.yaml`, written only by the CLI, records that modules were chosen and which are installed. It is specific to the installation and is not committed.
- **Compose:** each module's services belong to a compose profile named after the module. The CLI starts the stack with the profiles of installed modules only.
- **Gateway:** the CLI generates `deploy/Caddyfile` from the core routes plus the `apiPrefix` of every installed module, and the file is no longer committed.
- **Database provisioning:** provisioning becomes generic. Every installed module gets its role, schema, and secrets by convention from its id, instead of the gentasks-only list.
- **BREAKING (local setup):** on an existing checkout, the next `genwork up` shows the first-run checklist. Checking gentasks reuses its existing schema, role, and data.

## Components touched

cli, core, deploy, modules/gentasks (tests only).

The change spans core and the CLI because a removed module must disappear from clients (rules 56, 58), and only core publishes capabilities. Core gains two generic subcommands that know nothing about any specific module (rule 52). Every phase in `tasks.md` touches one component.

## Capabilities

### New Capabilities

- `module-catalog`: where the CLI finds modules and how the installation's choice is stored.
- `module-selection`: the first-run and `genwork modules` checklists, dependency rules, installing, uninstalling, and stack commands following the installation.
- `core-module-state`: the core subcommands that enable and disable a registered module, and their effect on capabilities.

### Modified Capabilities

_(none in `openspec/specs/`: the gateway and provisioning requirements this change refines belong to the `gentasks-module` change, which is not archived yet. The new specs state the behavior that replaces them: generated routes, and provisioning of installed modules only.)_

## Non-goals

- Non-interactive module commands for scripts and CI. The checklist is the only interface for now.
- Disabling a module while keeping its data. Unchecking always uninstalls.
- Pulling released module images or reading a remote module registry. The catalog is the repository's `modules/*/module.yaml`.
- Upgrading installed modules to newer versions.
- Generating `docker-compose.yml` itself. Module services are still written by hand, now under a profile, and a test keeps them in step with the catalog.
- Native client changes. They already follow capabilities (rule 58).

## Impact

- **Code:** new in the CLI: `cli/internal/modules` (catalog, installation config, selection plan, naming conventions, Caddyfile rendering), the `genwork modules` command, and the first-run step in `up`/`api`. Generic provisioning and deprovisioning replace the gentasks-only list. New in core: `core/internal/command` gains `module enable|disable`, and the `modules` service and repository gain a method that sets the enabled state.
- **Deploy:** gentasks services move under a `gentasks` profile, and the gateway no longer `depends_on` gentasks. `deploy/Caddyfile` and `deploy/genwork.yaml` are generated and gitignored. The gentasks test that read the committed Caddyfile is replaced by a CLI test.
- **Dependencies:** the CLI adds `github.com/charmbracelet/huh` (checklist and confirmation) and uses `sdk/go/manifest` through a `replace` directive, so `module.yaml` is parsed in one place only (rule 6).
- **Configuration:** module variables follow one convention from the module id: `<ID>_DB_PASSWORD`, `<ID>_DATABASE_URL`, `<ID>_UPSTREAM`. Those for gentasks already exist.
- **Build/CI:** CLI CI also runs when `sdk/**`, `modules/*/module.yaml`, or `deploy/docker-compose.yml` change.
