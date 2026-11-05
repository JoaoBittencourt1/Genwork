## Purpose

Defines the genagents screens in the web app: plain, functional pages for accounts, bots, skills, MCP servers, runners, and the admin chat.

## ADDED Requirements

### Requirement: Navigation follows capabilities and permissions
The web app SHALL show an "AI" section only when genagents is enabled (rule 58). Inside it, each page SHALL be shown only to callers holding its permission: "Accounts" (`genagents.account.manage`), "Bots" (`genagents.bot.read`; editing needs `genagents.bot.manage`), "Skills" (`genagents.skill.manage`), "MCP servers" (`genagents.mcpserver.manage`), "Runners" (`genagents.runner.manage`), and "Admin chat" (`genagents.adminchat.use`).

#### Scenario: Module disabled
- **WHEN** genagents is disabled
- **THEN** no AI section, page, or link is shown anywhere

#### Scenario: Member sees bots read-only
- **WHEN** a member with `genagents.bot.read` only opens "Bots"
- **THEN** the list is shown without create, edit, or delete actions

### Requirement: Management pages
"Accounts", "Bots", "Skills", "MCP servers", and "Runners" SHALL each be a table with a create button and a form dialog for create and edit, built from the shared shadcn components (rule 73). The account form SHALL show the runner sign-in command for subscription accounts; the runner form SHALL show the register command with the one-time token; the bot form SHALL offer models filtered by the chosen account.

#### Scenario: Bot form filters models
- **WHEN** an admin picks a Cursor account in the bot form
- **THEN** the model list shows only Cursor models and that account's custom models

#### Scenario: One-time token shown once
- **WHEN** an admin closes the runner registration dialog
- **THEN** the token cannot be shown again; a new runner or a new registration token must be created

### Requirement: Admin chat page
The admin chat page SHALL list the admin's chats on the side and show the selected chat with its messages, each assistant turn's tool activity in a collapsible list, a message box, and a "Stop" button while a turn runs. It SHALL poll for new entries every 2 seconds while a turn is active.

#### Scenario: Turn in progress
- **WHEN** the admin sends a message
- **THEN** the turn shows as in progress with its tool activity growing until the answer arrives

### Requirement: Copy
Every user-facing string SHALL come from message keys with English and pt-BR translations (rule 36).

#### Scenario: Portuguese locale
- **WHEN** the locale is pt-BR
- **THEN** every genagents page and dialog is shown in Portuguese
