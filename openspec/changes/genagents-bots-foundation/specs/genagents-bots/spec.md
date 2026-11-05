## Purpose

Lets an admin define bots, each a configured AI worker with a kind, an account and model, parameters, skills, and MCP servers, backed by its own core bot member.

## ADDED Requirements

### Requirement: Bot definition
A caller holding `genagents.bot.manage` SHALL be able to create a bot with: name, kind (`dev`, `reviewer`, or `assistant`), account, model, role for its bot member, instructions (free text added to every run), parameters, skills, and MCP servers. Parameters SHALL be: reasoning effort (`low`, `medium`, `high`, or provider default), maximum turns (1–500), and run timeout (5–720 minutes). A model that is not in the account's catalog or custom models SHALL be rejected with 400.

#### Scenario: Create a dev bot
- **WHEN** an admin creates "Dev Bot" of kind `dev` on "Claude — Team" with model `claude-sonnet-5-5`
- **THEN** the bot is listed with that kind, account, and model

#### Scenario: Model from another provider
- **WHEN** an admin chooses an OpenAI model for a bot on an Anthropic account
- **THEN** the request is rejected with 400

### Requirement: Bot member lifecycle
Creating a bot SHALL create its core bot member with the bot's name and chosen role, and genagents SHALL keep the member's client secret encrypted. Renaming the bot SHALL rename the member; changing the role SHALL change the member's role; disabling the bot SHALL deactivate the member; deleting the bot SHALL remove the member. If core refuses the member operation, the bot operation SHALL fail and leave no partial state.

#### Scenario: Bot appears as a member
- **WHEN** "Dev Bot" is created with role `member`
- **THEN** a bot member "Dev Bot" with role `member` exists in core

#### Scenario: Disabled bot
- **WHEN** an admin disables "Dev Bot"
- **THEN** its bot member is deactivated, it cannot be assigned new work, and its running runs are cancelled

### Requirement: Kinds decide what a bot can do
Only `dev` bots SHALL be startable by task assignment. Only `reviewer` bots SHALL be offered for bot reviews. `assistant` bots SHALL be offered in the admin chat. Changing a bot's kind SHALL NOT affect runs already started.

#### Scenario: Reviewer assigned to a task
- **WHEN** a task is assigned to a `reviewer` bot's member
- **THEN** the assignment is kept but no run starts, and the task conversation explains that only dev bots work tasks

### Requirement: Partial updates
`PATCH` on a bot SHALL follow rule 17: omitted fields unchanged, `null` clears nullable fields (instructions) and is rejected for required ones.

#### Scenario: Single-field update
- **WHEN** an admin patches only the model of "Dev Bot"
- **THEN** every other field keeps its stored value

### Requirement: Reading bots
A caller holding `genagents.bot.read` SHALL be able to list active bots with name, kind, provider, model, and member token, never secrets. Members use this to pick reviewers and to recognize bot assignees.

#### Scenario: Member lists reviewers
- **WHEN** a member lists bots of kind `reviewer`
- **THEN** each has its name, provider, model, and member token, and no credentials
