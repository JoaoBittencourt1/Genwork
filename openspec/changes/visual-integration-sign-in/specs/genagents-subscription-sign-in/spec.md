## Purpose

Signs subscription accounts in from the web app by driving the provider CLI's own sign-in on the runner that holds the account, without a terminal on that machine.

## ADDED Requirements

### Requirement: Start a sign-in
A caller holding `genagents.account.manage` SHALL start a sign-in for a subscription account with `POST /api/genagents/v1/accounts/{token}/sign-ins`. Starting one SHALL cancel the account's active sign-in. An API-key account SHALL be refused with a validation error on `mode`.

#### Scenario: Claude subscription account
- **WHEN** an admin clicks "Log in with Claude" on a subscription account
- **THEN** a sign-in in state `requested` is created and returned in the account's `signIn` field

#### Scenario: API-key account
- **WHEN** an admin starts a sign-in for an API-key account
- **THEN** the request fails with 400 and field `mode`

### Requirement: Runner drives the CLI sign-in
The runner holding the account SHALL pick up a requested sign-in within seconds and run the provider CLI's sign-in in a sandbox that mounts only that account's profile. It SHALL report the authorization URL and any device code, type a submitted code into the CLI, and report success or failure. Provider credentials SHALL stay in the CLI's profile on the runner.

#### Scenario: URL relayed
- **WHEN** the CLI prints its authorization URL
- **THEN** the sign-in moves to `awaiting_user` and the web app shows the URL and its host

#### Scenario: Code pasted
- **WHEN** an admin pastes the code from Anthropic's page into Genwork
- **THEN** the runner receives it once, the stored copy is cleared when the runner reports `verifying`, and the account becomes `ready` when the CLI exits successfully

### Requirement: Secrets never leave the flow
No response to a person, log line, or MCP tool SHALL contain a pasted code or a provider token. The pasted code SHALL be stored only encrypted and only until the runner takes it.

#### Scenario: Account listing
- **WHEN** an admin lists accounts during a sign-in with a submitted code
- **THEN** the response has no code value

### Requirement: Visible outcomes
A cancelled, expired, or failed sign-in SHALL be visible on the account. Expiry or failure SHALL move the account to `error` with a message. A sign-in SHALL expire 10 minutes after it is requested.

#### Scenario: Expired
- **WHEN** nobody completes a sign-in within 10 minutes
- **THEN** the sign-in reads `expired` and the account is `error` with `sign_in_expired`

#### Scenario: Cancelled
- **WHEN** an admin cancels a sign-in
- **THEN** it reads `cancelled`, the runner stops the CLI, and the account keeps its previous status

### Requirement: Fallback command
The account response SHALL keep the `genwork-runner account login` command for a pending account, and the web app SHALL offer it as a secondary option.

#### Scenario: Fallback shown
- **WHEN** an account is `pending_sign_in`
- **THEN** the web app shows the login button first and the command as an alternative
