## Purpose

Gives the installer a generic way to switch a registered module on or off in core, so that when a module is uninstalled or installed again, the capabilities endpoint and every client that follows it stop or resume showing it (rules 56, 58).

## ADDED Requirements

### Requirement: Enable and disable subcommands
Core SHALL provide `core module enable <id>` and `core module disable <id>`, run inside the core container with core's own database configuration. They set the stored enabled state of the registered module with that id and print the result. Neither subcommand contains logic specific to any module (rule 52).

#### Scenario: Disable a registered module
- **WHEN** `core module disable gentasks` runs while gentasks is registered and enabled
- **THEN** gentasks is stored as disabled, and the command exits with status 0

#### Scenario: Unknown module
- **WHEN** the id was never registered with core
- **THEN** the command prints that there is nothing to change and exits with status 0, because the module will register itself when it starts

#### Scenario: Missing argument
- **WHEN** the module id is missing
- **THEN** the command prints its usage and exits with a non-zero status

### Requirement: Capabilities follow the stored state
`GET /api/core/v1/capabilities` SHALL list only modules stored as enabled. A module re-registering on startup SHALL NOT change its stored enabled state.

#### Scenario: Disabled then restarted
- **WHEN** gentasks is disabled in core and its container still starts and registers
- **THEN** capabilities still do not list gentasks

#### Scenario: Re-enabled
- **WHEN** `core module enable gentasks` runs
- **THEN** capabilities list gentasks again on the next request
