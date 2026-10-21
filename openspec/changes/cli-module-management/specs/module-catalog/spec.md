## Purpose

Defines where the installer CLI finds the modules an installation can choose from, and how it records the installation's choice.

## ADDED Requirements

### Requirement: Catalog from module manifests
The CLI SHALL build the module catalog from every `modules/<name>/module.yaml` in the workspace, parsed and validated with the same rules modules use for registration. A manifest that fails validation SHALL be reported by path and left out of the catalog without hiding the valid ones.

#### Scenario: Valid manifests
- **WHEN** the workspace has `modules/gentasks/module.yaml`
- **THEN** the catalog contains gentasks with its version, API prefix, and required modules

#### Scenario: Invalid manifest
- **WHEN** one `module.yaml` has an invalid version
- **THEN** the CLI prints a warning naming that file, and every other module is still offered

### Requirement: Installation config
The CLI SHALL record the installation's choice in `deploy/genwork.yaml`: the list of installed module ids. Only the CLI writes this file. A missing file means the modules were never chosen. An existing file with an empty list means the installation chose no modules. The file is specific to the installation and SHALL NOT be committed.

#### Scenario: Never chosen
- **WHEN** `deploy/genwork.yaml` does not exist
- **THEN** the next `genwork up` or `genwork api` shows the first-run checklist

#### Scenario: Chose nothing
- **WHEN** `deploy/genwork.yaml` exists with no modules
- **THEN** `genwork up` starts no module containers and does not show the checklist

#### Scenario: Malformed config
- **WHEN** `deploy/genwork.yaml` cannot be parsed
- **THEN** `genwork up`, `genwork api`, and `genwork modules` fail with a message naming the file, and change nothing
