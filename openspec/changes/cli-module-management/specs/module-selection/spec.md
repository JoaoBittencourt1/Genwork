## Purpose

Lets people choose an installation's modules through a checklist, on the first start and at any time afterwards: a checked module is installed, an unchecked one is not. Dependencies are honored, and data is never removed without explicit confirmation (rules 55, 56, 64).

## ADDED Requirements

### Requirement: Module checklist
The checklist SHALL list every catalog module sorted by id, each with its id, version, and required modules, and with installed modules pre-checked. The space bar toggles the focused module and Enter confirms. An installed module that is missing from the catalog SHALL still be listed, checked, and marked as missing, so it can be uninstalled. Without an interactive terminal, the checklist SHALL NOT be shown: the command explains that it needs a terminal and changes nothing.

#### Scenario: Installed modules are checked
- **WHEN** gentasks is installed and `genwork modules` runs
- **THEN** gentasks appears checked, and modules that are not installed appear unchecked

#### Scenario: No terminal
- **WHEN** `genwork modules` runs with stdin not attached to a terminal
- **THEN** it fails with a message that it needs an interactive terminal, and nothing changes

### Requirement: First-run checklist
When `genwork up` or `genwork api` runs and `deploy/genwork.yaml` does not exist, the CLI SHALL show the checklist with every module unchecked before starting the stack. It applies the confirmed selection and saves the choice, even an empty one. Without an interactive terminal, the CLI SHALL save an empty choice, print that modules can be chosen later with `genwork modules`, and start the stack.

#### Scenario: First start
- **WHEN** a new installation runs `genwork up` and checks gentasks
- **THEN** gentasks is installed, the stack starts with it, and the next `genwork up` does not ask again

#### Scenario: First start without a terminal
- **WHEN** the first `genwork up` runs without a terminal
- **THEN** no module is installed, `deploy/genwork.yaml` is saved with no modules, and the hint to run `genwork modules` is printed

### Requirement: Selection plan and confirmation
After the checklist is confirmed, the CLI SHALL compare the selection with the installed modules. It then shows a summary of the modules to install (with any added dependencies and why) and the modules to uninstall, and applies nothing until the summary is confirmed. When the plan uninstalls any module, the confirmation SHALL state that the module's data will be permanently deleted, and SHALL default to "no". A selection equal to the installed set changes nothing and says so.

#### Scenario: Nothing changed
- **WHEN** the user confirms the checklist without changing any box
- **THEN** the CLI prints that there is nothing to change and exits with status 0

#### Scenario: Uninstall declined
- **WHEN** the plan uninstalls gentasks and the user answers "no" to the confirmation
- **THEN** nothing is installed or uninstalled

### Requirement: Dependencies
Checking a module SHALL also install every module it requires, recursively. The plan SHALL be refused, with nothing changed, when it would uninstall a module that a module staying installed requires (naming that module), when a required module is not in the catalog, when the catalog version does not satisfy the required range, or when requirements form a cycle.

#### Scenario: Required module added
- **WHEN** module `reports` requires `gentasks` and only `reports` is checked
- **THEN** the summary lists gentasks as added because reports requires it, and both are installed

#### Scenario: Uninstalling a required module
- **WHEN** `reports` stays checked, requires gentasks, and gentasks is unchecked
- **THEN** the CLI refuses naming `reports`, and nothing changes

#### Scenario: Unsatisfiable range
- **WHEN** a checked module requires `gentasks` `>=2.0.0` and the catalog has gentasks `0.1.0`
- **THEN** the CLI refuses naming both modules and the range, and nothing changes

### Requirement: Installing a module
Installing a module SHALL, in order:
1. add it to `deploy/genwork.yaml`
2. generate any missing `<ID>_DB_PASSWORD` and `<ID>_DATABASE_URL` values in `deploy/.env`
3. provision its database role and schema
4. regenerate the gateway routes
5. run its migrations and start its containers
6. run `core module enable <id>`
7. restart the gateway

Required modules are installed before the modules that need them. These steps work the same whether or not the stack is running. A failed step leaves the module recorded as installed, and rerunning `genwork modules` or `genwork up` completes it.

#### Scenario: Install into a running stack
- **WHEN** core is running and gentasks is checked in `genwork modules`
- **THEN** gentasks starts, registers with core, appears in capabilities, and answers on `/api/gentasks/v1/...` through the gateway, without core being restarted

#### Scenario: Reinstall after uninstall
- **WHEN** gentasks was uninstalled earlier and is checked again
- **THEN** it starts with an empty schema and appears in capabilities again

### Requirement: Uninstalling a module
Uninstalling a module SHALL, in order:
1. stop and remove its containers
2. run `core module disable <id>`
3. drop its database schema (cascading to every table) and its role
4. clear its `<ID>_DB_PASSWORD` and `<ID>_DATABASE_URL` values
5. remove it from `deploy/genwork.yaml`
6. regenerate the gateway routes
7. restart the gateway

Modules that depend on others are uninstalled before the modules they depend on. Every step succeeds when there is nothing to do, so a failed uninstall is completed by unchecking the module again. This physical deletion is the documented exception to soft delete for a module's data (rules 22, 56).

#### Scenario: Uninstall removes the module
- **WHEN** gentasks is unchecked and the deletion is confirmed
- **THEN** its containers are gone, capabilities no longer list it, `/api/gentasks/v1/...` answers `404` through the gateway, and the `gentasks` schema and role no longer exist

### Requirement: Stack commands follow the installation
`genwork up` and `genwork api` SHALL start only the containers of installed modules, using one compose profile per module named after its id. They SHALL provision every installed module's database and regenerate the gateway routes before starting the gateway. `genwork down` SHALL stop the containers of every module profile.

#### Scenario: Module not installed
- **WHEN** gentasks is not installed and `genwork up` runs
- **THEN** the gentasks containers are not started and the gateway has no gentasks route

### Requirement: Generated gateway routes
`deploy/Caddyfile` SHALL be generated by the CLI. It keeps the core routes and the `/internal/*` block, and adds one route per installed module from its manifest `apiPrefix`, with the module's upstream read from the `<ID>_UPSTREAM` variable. It is regenerated on every stack start and after every change of modules, and SHALL NOT be committed.

#### Scenario: Route follows installation
- **WHEN** gentasks is installed
- **THEN** the generated Caddyfile routes `/api/gentasks/v1/*` to `{$GENTASKS_UPSTREAM}`, and after gentasks is uninstalled the route is gone

### Requirement: Compose services match the catalog
Every catalog module SHALL have a `<id>` service and a `<id>-migrate` service in `deploy/docker-compose.yml`, both in the `<id>` profile. No service outside a module's profile may depend on a module service.

#### Scenario: Consistency check
- **WHEN** the CLI test suite runs
- **THEN** it fails if any catalog module lacks its two services or profile, or if the gateway depends on a module service
