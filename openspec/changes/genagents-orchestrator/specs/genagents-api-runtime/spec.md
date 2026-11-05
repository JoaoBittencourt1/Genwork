## Purpose

Runs chat, planning, and review work directly against provider APIs from the genagents service when an account uses an API key, so those runs need no runner.

## ADDED Requirements

### Requirement: Direct API execution
For an `api_key` account whose provider has an API runtime, runs of kind `chat`, `review`, and manager heartbeats SHALL execute inside genagents through the provider's API with a tool loop over the Genwork MCP and the bot's HTTP MCP servers. Runs that need a repository workspace (`task` runs of dev bots) SHALL keep using runners. A bot SHALL choose `runner` or `api` execution where both are possible; the default is `api` for API-key accounts.

#### Scenario: Admin chat without a runner
- **WHEN** an admin chats with an assistant bot on an Anthropic API-key account and no runner is online
- **THEN** the chat answers through the API runtime

#### Scenario: Dev run still needs a runner
- **WHEN** a task is assigned to a dev bot on an API-key account and no runner is online
- **THEN** the run stays queued and the conversation says it waits for a runner

### Requirement: Same contract as runner runs
API runtime runs SHALL produce the same run states, conversation entries, usage records, and audit entries as runner runs, so clients cannot tell them apart except by an `execution` field.

#### Scenario: Conversation looks the same
- **WHEN** a review runs through the API runtime
- **THEN** its conversation entries have the same types and shape as a runner review
