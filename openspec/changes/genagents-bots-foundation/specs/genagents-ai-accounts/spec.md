## Purpose

Lets an admin connect the team's AI provider accounts to Genwork, through a subscription plan signed in on a runner or through an API key, with any number of accounts per provider.

## ADDED Requirements

### Requirement: Providers
genagents SHALL support these providers, each a fixed value (rule 15): `anthropic` (Claude, run through Claude Code), `openai` (ChatGPT, run through Codex), `cursor` (run through the Cursor agent CLI), and `xai` (Grok, run through the Grok CLI). Each provider SHALL declare which account modes it supports and its model catalog.

#### Scenario: Provider list
- **WHEN** an admin opens the new account form
- **THEN** the four providers are offered, each with the modes it supports

### Requirement: Link an account
A caller holding `genagents.account.manage` SHALL be able to link an account with a display name, a provider, and a mode: `subscription` or `api_key`. Several accounts of the same provider SHALL be allowed, and display names SHALL be unique among active accounts. An `api_key` account SHALL require the key, which genagents SHALL store encrypted and never return in any response. A `subscription` account SHALL require a runner, and SHALL stay `pending_sign_in` until that runner reports a successful sign-in for it.

#### Scenario: Two Claude accounts
- **WHEN** an admin links "Claude — João" and "Claude — Team", both `anthropic`
- **THEN** both are listed as separate accounts and either can be chosen for a bot

#### Scenario: API key is never returned
- **WHEN** any account is read
- **THEN** the response shows whether a key is set and its last four characters, never the key

#### Scenario: Subscription account waits for sign-in
- **WHEN** an admin links a `subscription` account on runner "build-box"
- **THEN** its status is `pending_sign_in` and the screen shows the runner command that signs it in

#### Scenario: Without permission
- **WHEN** a member without `genagents.account.manage` tries to link an account
- **THEN** the request is rejected with 403

### Requirement: Account status
Every account SHALL have a status: `pending_sign_in`, `ready`, or `error` with the last error message. A runner SHALL report the status of the subscription accounts it holds; an `api_key` account SHALL be `ready` once its key is set and SHALL become `error` when a run fails with an authentication error from the provider.

#### Scenario: Sign-in completed
- **WHEN** the runner reports a successful sign-in for a pending account
- **THEN** the account becomes `ready`

#### Scenario: Revoked key
- **WHEN** a run on an `api_key` account fails because the provider rejects the key
- **THEN** the account becomes `error` with the provider's message, and the run fails with that reason

### Requirement: Update and remove accounts
An admin SHALL be able to rename an account, replace its API key, and remove it. Removing an account used by a bot SHALL be refused with the list of bots using it. Removing a subscription account SHALL tell its runner to delete the account's sign-in profile.

#### Scenario: Account in use
- **WHEN** an admin removes an account that "Dev Bot" uses
- **THEN** the request is rejected with 409 naming "Dev Bot"

### Requirement: Model catalog
Each provider SHALL expose a model catalog (id and display name). An admin SHALL be able to add a custom model id to an account, for models released after the catalog. A bot's model SHALL be chosen from its account's provider catalog plus that account's custom models.

#### Scenario: Custom model
- **WHEN** an admin adds custom model `claude-opus-5-5` to a Claude account
- **THEN** that model can be selected for bots on that account
