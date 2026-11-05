## Purpose

Gives every bot run access to Genwork through the Genwork MCP server under the right identity, and lets an admin register external MCP servers and attach them to bots.

## ADDED Requirements

### Requirement: Genwork MCP in every run
Every run SHALL be configured with the instance's Genwork MCP server. Task runs and reviews SHALL authenticate as the bot's member; admin chat runs SHALL authenticate as the admin through a delegated session. Access tokens SHALL be short-lived and renewed by the runner through genagents while the run lasts, and SHALL stop being issued once the run ends.

#### Scenario: Bot acts as itself
- **WHEN** a dev bot moves its task through the Genwork MCP
- **THEN** gentasks records the move as made by the bot member

#### Scenario: Token after the run ends
- **WHEN** a runner asks for a new Genwork token for a run that has finished
- **THEN** the request is rejected

### Requirement: External MCP servers
A caller holding `genagents.mcpserver.manage` SHALL be able to register an external MCP server with a name (unique among active servers), transport (`http` or `stdio`), and either a URL with optional headers (`http`) or a command with arguments and environment (`stdio`). Header and environment values marked secret SHALL be stored encrypted and never returned. A `stdio` server SHALL run on the runner machine, which needs the command installed.

#### Scenario: Register an HTTP server
- **WHEN** an admin registers "sentry" with URL `https://mcp.sentry.dev/mcp` and a secret `Authorization` header
- **THEN** reads show the header name and that a value is set, never the value

### Requirement: Attaching servers to bots
A bot SHALL have zero or more external MCP servers. Each run SHALL receive the Genwork MCP plus the bot's servers. Removing a server SHALL detach it from every bot.

#### Scenario: Run with an extra server
- **WHEN** "Dev Bot" has "sentry" attached and starts a run
- **THEN** the CLI is configured with the Genwork MCP and "sentry"
