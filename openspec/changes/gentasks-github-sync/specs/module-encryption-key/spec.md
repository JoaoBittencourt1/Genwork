## Purpose

Gives every installed module its own encryption key, provisioned by the installer, so a module can keep secrets it receives at runtime encrypted at rest without manual setup.

## ADDED Requirements

### Requirement: Generate a per-module encryption key
When the installer ensures a module's secrets, it SHALL set `<MODULE>_ENCRYPTION_KEY` (the module id upper-cased, hyphens as underscores) in the stack `.env` to 256 fresh random bits encoded as unpadded base64url, if the variable is missing or empty. It SHALL never overwrite an existing value, because rotating it would make the module's stored secrets unreadable. The variable name SHALL be derived from the module id by convention, so no module-specific installer code is needed.

#### Scenario: Fresh install
- **WHEN** `gentasks` is installed and `GENTASKS_ENCRYPTION_KEY` is absent from `.env`
- **THEN** the installer writes a 43-character base64url value to `GENTASKS_ENCRYPTION_KEY`

#### Scenario: Existing installation upgraded
- **WHEN** `genwork up` runs on an installation whose `.env` predates the variable
- **THEN** the variable is added with a fresh value and every other value is left untouched

#### Scenario: Existing key kept
- **WHEN** `GENTASKS_ENCRYPTION_KEY` already has a value
- **THEN** the installer leaves it unchanged

### Requirement: Clear the key on uninstall
Uninstalling a module SHALL empty its `<MODULE>_ENCRYPTION_KEY` together with its database credentials, so a reinstall starts with a fresh key and fresh data.

#### Scenario: Uninstall
- **WHEN** `gentasks` is uninstalled
- **THEN** `GENTASKS_ENCRYPTION_KEY`, `GENTASKS_DB_PASSWORD`, and `GENTASKS_DATABASE_URL` are emptied in `.env`
