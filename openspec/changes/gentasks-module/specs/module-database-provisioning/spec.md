## Purpose

Ensures each module gets its own Postgres schema and a database role that cannot read any other schema (rule 53), and that the secrets this needs are generated rather than hardcoded.

## ADDED Requirements

### Requirement: Generate module secrets on first run
When `genwork up`, `api`, or `db` runs and `deploy/.env` lacks `GENWORK_MODULE_SECRET` or `GENTASKS_DB_PASSWORD`, the CLI SHALL generate each from a cryptographically secure random source (at least 128 bits) and write it into `deploy/.env`, leaving existing values untouched.

#### Scenario: Fresh checkout
- **WHEN** `deploy/.env` is copied from `.env.example` with both values empty and `genwork up` runs
- **THEN** both values are filled with random, URL-safe strings

#### Scenario: Values already set
- **WHEN** both values are already present
- **THEN** the CLI does not change them

### Requirement: Provision the module role and schema idempotently
Before starting module containers, the CLI SHALL ensure that a login role named after the module exists with the configured password, that a schema of the same name exists and is owned by that role, and that the role has no privileges on any other schema.

#### Scenario: First start
- **WHEN** the database has no `gentasks` role
- **THEN** the CLI creates the role and the `gentasks` schema, and the stack starts

#### Scenario: Second start
- **WHEN** the role and schema already exist
- **THEN** the CLI changes nothing and does not fail

#### Scenario: Isolation
- **WHEN** a session connected as `gentasks` selects from `core.users`
- **THEN** Postgres refuses with a permission error
