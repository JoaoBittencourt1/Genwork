## ADDED Requirements

### Requirement: Create administrator from the CLI
`genwork auth` SHALL ask for the e-mail and the name (re-asking while blank, or taking them from `--email` and `--name` when given), prompt for a password twice without echoing it, validate that both entries match, and create an active user with the `admin` role by running `core admin create` inside the running stack, passing the password through standard input. The password SHALL never appear in command-line arguments, environment variables, or logs.

#### Scenario: First administrator
- **WHEN** an operator runs `genwork auth`, answers `ana@example.com` and `Ana`, and enters the same valid password twice
- **THEN** an active `admin` user with that e-mail exists and can sign in with that password

#### Scenario: Passwords do not match
- **WHEN** the two password entries differ
- **THEN** the CLI exits with a non-zero status without creating anything

#### Scenario: E-mail already registered
- **WHEN** the e-mail already belongs to a user
- **THEN** `core admin create` exits with a non-zero status, reports `email_already_registered` with a readable explanation, and changes nothing

#### Scenario: Stack not running
- **WHEN** the CLI cannot reach the core container
- **THEN** it exits with a non-zero status and tells the operator to start it with `genwork api` or `genwork up`

### Requirement: Local stack commands
The CLI SHALL find the repository root from any subdirectory and offer `genwork db` (PostgreSQL), `genwork api` (core with its database, migrations, worker, and mail catcher), `genwork web` (web dev server in the foreground, installing dependencies when missing), `genwork up` (everything, with the web app in the foreground), and `genwork down` (stop containers, keep data). Starting commands SHALL create missing `.env` files from their examples, generating a new JWT signing key, and SHALL never overwrite existing ones.

#### Scenario: First run
- **WHEN** a developer without `deploy/.env` runs `genwork up`
- **THEN** `deploy/.env` is created with a random 32-byte signing key, `web/.env` is created, the containers start, and the web dev server runs

#### Scenario: Outside the repository
- **WHEN** a stack command runs outside a Genwork repository
- **THEN** it exits with a non-zero status explaining that no repository was found

### Requirement: No public bootstrap path
Core SHALL expose no HTTP endpoint that creates a user without an invitation, regardless of whether any user exists.

#### Scenario: Empty instance
- **WHEN** the instance has no users and someone looks for a way to register over HTTP
- **THEN** the only way to create a user is the CLI or an invitation
