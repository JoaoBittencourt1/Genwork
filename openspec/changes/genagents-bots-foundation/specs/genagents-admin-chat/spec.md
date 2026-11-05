## Purpose

Gives admins, and only admins, a general-purpose chat with an AI that acts as them across Genwork through the Genwork MCP server.

## ADDED Requirements

### Requirement: Admin-only access
Every admin chat endpoint SHALL require `genagents.adminchat.use`, granted by default only to the `admin` role. The web app SHALL show the admin chat only to callers holding it. A chat SHALL be visible only to the admin who created it.

#### Scenario: Member opens the chat
- **WHEN** a member without `genagents.adminchat.use` requests the admin chat
- **THEN** the request is rejected with 403 and the navigation does not show it

#### Scenario: Another admin's chat
- **WHEN** an admin reads a chat created by another admin
- **THEN** the request is rejected with 404

### Requirement: Chats
An admin SHALL be able to start a chat choosing an enabled `assistant` bot (its account, model, instructions, skills, and MCP servers apply), list their chats, rename them, and delete them (soft delete). Each message SHALL run as one turn on the bot's account, resuming the chat's CLI session so the AI keeps context.

#### Scenario: Start and continue
- **WHEN** an admin starts a chat with "Ops Assistant" and sends two messages
- **THEN** the second answer takes the first exchange into account

### Requirement: Acts as the admin
Starting a chat SHALL grant a delegated session from the admin to genagents with purpose "Admin chat"; the chat's Genwork MCP SHALL authenticate through it, so the AI can do everything the admin can, and nothing more. Ending the session from the admin's session list SHALL make the chat's next turn fail with a message asking the admin to reopen the chat, which grants a new session. Deleting the chat SHALL revoke its session.

#### Scenario: Reach across modules
- **WHEN** an admin asks "list the tasks assigned to me and the host CPU usage"
- **THEN** the AI answers using the gentasks and genmonitor tools available to the admin through the Genwork MCP

#### Scenario: Admin demoted
- **WHEN** the admin's role is changed to `member` while the chat exists
- **THEN** the next turn can no longer use admin-only tools, and the chat endpoints reject the former admin with 403

### Requirement: Genwork coverage
The Genwork MCP SHALL expose tools for every enabled component so the admin chat reaches all of Genwork: the existing core and gentasks tools, plus new genmonitor tools (read host snapshots) and genagents tools (list bots, list and read runs and conversations, post a message to a task conversation, request a bot review). Each tool SHALL keep the existing permission filter, so the AI sees only what the admin's role allows.

#### Scenario: genmonitor enabled
- **WHEN** genmonitor is enabled and an admin's chat lists its tools
- **THEN** the genmonitor host tools are available

### Requirement: Tool activity is visible
Each assistant turn SHALL show, alongside its answer, the tools it called in order with a one-line summary each, so the admin sees what the AI did on their behalf.

#### Scenario: Write action shown
- **WHEN** the AI creates a task during a turn
- **THEN** the turn lists the task creation tool call with the new task key
