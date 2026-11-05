## Why

Genwork already lets a person drive the instance from an AI assistant (the MCP server), but the AI never works *inside* Genwork: an admin cannot connect the team's AI accounts, define bots, hand a task to a bot, or ask a bot to review a task's pull request. This change is the first delivery of the AI orchestrator described in `genagents-orchestrator`: it makes bots real, assignable, and reviewable, keeps the design deliberately plain, and lays the identity and execution foundations the orchestrator builds on.

## What Changes

- **New module `genagents`** (rule 49) that owns AI accounts, bots, skills, external MCP servers, runners, runs, and conversations.
- **AI accounts.** An admin links any number of AI accounts, several per provider allowed (two Claude accounts side by side is a supported case). Providers: Anthropic (Claude / Claude Code), OpenAI (ChatGPT / Codex), Cursor, and xAI (Grok). Each account is either a **subscription plan** (signed in on a runner through the provider's official CLI; Genwork never sees the credential) or an **API key** (stored encrypted by genagents, handed to a runner only for a run).
- **Bots.** An admin creates bots with: name, avatar initials, kind (`dev`, `reviewer`, or `assistant`), account + model (the model list comes from the chosen account's provider), skills, parameters (instructions, reasoning effort, max turns, run timeout), and extra MCP servers. Every bot is a **core bot member**, so it appears in member pickers and acts through the Genwork MCP with its own role and permissions.
- **Skills.** An admin creates skills in the Agent Skills format (`SKILL.md` with name, description, and body, plus optional extra text files) and attaches them to bots; the runner installs them where each CLI expects them.
- **MCP access.** Every run gets the Genwork MCP server, signed in as the bot member (or as the admin, for the admin chat), plus the external MCP servers attached to the bot.
- **Runners.** A new component, `runner/` (`genwork-runner`), runs on a machine the admin controls with the provider CLIs installed. It registers with a one-time token, holds subscription sign-ins in one isolated profile per account, claims runs atomically, executes the CLI in an isolated workspace, and streams status back.
- **Task runs.** Assigning a gentasks task to a `dev` bot starts a run: the bot clones the repository, works the task, moves it on the board through the MCP, and opens a pull request that cites the task. A **conversation panel** on the task shows the bot's status updates and lets the person who assigned it (and anyone with access) talk to the bot while it works.
- **Bot review.** A **"Bot review"** button on the task dialog asks a `reviewer` bot to review the pull requests linked to the task (diff + task description); the bot posts a summary in the task conversation and a review with comments on the GitHub pull request.
- **Admin chat.** A chat available **only to admins** where they talk to an AI normally; the AI acts **as that admin** through the Genwork MCP, so it can reach everything the admin can.
- **Core** gains two generic extension points (rule 52): **bot members** (non-human members that cannot sign in with a password and authenticate with client credentials) and **delegated sessions** (a revocable session a user grants to a module for a bounded purpose, used by the admin chat).
- **gentasks** accepts bot members as assignees, notifies genagents when a task is assigned to a bot (optional integration, rule 55), and exposes the bot review request.
- **Web**: plain, functional screens: AI accounts, bots, skills, MCP servers, runners, admin chat, plus the conversation panel and the "Bot review" button in the gentasks task dialog.
- **Constitution**: adds `runner/` to the component table.

Components touched: **core** (bot members, delegated sessions), **genagents module** (new), **gentasks module** (bot assignees, notification, review request), **runner** (new component), **mcp** (bot-member sessions, genagents tools), **cli** (installer lists the module, release ships the runner), **web**. The macOS, Windows, and Linux clients are not touched. The change spans core and two modules because bots must be members (core owns members, rule 49), work happens on tasks (gentasks), and everything else is the new module's; phases are split per component (rule 63).

### `modules/genagents/module.yaml`

- `id: genagents`, `core: ">=1.4.0 <2.0.0"` (needs bot members and delegated sessions), `apiPrefix: /api/genagents/v1`
- `requires: []`. gentasks is an optional integration in both directions: without gentasks, bots and the admin chat still work; without genagents, gentasks hides bot features.
- Permissions: `genagents.account.manage` [admin], `genagents.bot.read` [admin, member], `genagents.bot.manage` [admin], `genagents.skill.manage` [admin], `genagents.mcpserver.manage` [admin], `genagents.runner.manage` [admin], `genagents.run.read` [admin, member], `genagents.run.write` [admin, member], `genagents.adminchat.use` [admin]
- Events: publishes none, consumes none (the event bus is still an open decision; module-to-module calls use versioned HTTP APIs, rule 54).

## Capabilities

### New Capabilities

- `core-bot-members`: non-human members, their client-credential authentication, and how they appear in member and directory APIs.
- `core-delegated-sessions`: a user grants a module a bounded, revocable session that acts as that user.
- `genagents-ai-accounts`: linking subscription and API-key accounts per provider, several per provider, and the provider model catalog.
- `genagents-bots`: bot definition (kind, account, model, parameters, skills, MCP servers) and its bot member.
- `genagents-skills`: skill authoring in the Agent Skills format and attaching skills to bots.
- `genagents-mcp-servers`: external MCP servers an admin registers and attaches to bots, and the Genwork MCP every run receives.
- `genagents-runners`: runner registration, subscription sign-in per account, atomic run claiming, run execution, and status streaming.
- `genagents-task-runs`: dev runs started by task assignment and the task conversation.
- `genagents-bot-reviews`: reviewer runs over a task's linked pull requests.
- `genagents-admin-chat`: the admin-only chat that acts as the admin.
- `genagents-web-ui`: the module's screens.
- `gentasks-bot-assignment`: bot assignees, the assignment notification to genagents, the bot review request, and the task dialog additions.

### Modified Capabilities

None in `openspec/specs/` (no capability is archived yet). This change extends behavior introduced by `gentasks-module` (task assignee) and `member-management` (member list) without changing their existing requirements: humans keep working exactly as before.

## Non-goals

Everything the orchestrator adds later (see `genagents-orchestrator`): manager bots and delegation, heartbeats and routines, OKRs and cycles, budgets and cost tracking, approvals, the audit log, and running models through the provider APIs without a runner. Also out of this delivery:

- Polished design: screens use the existing shadcn components with minimal layout.
- Server push: the conversation panel polls.
- Running several bots on the same task at once, or bots reassigning tasks to other bots.
- GitHub repositories outside the gentasks GitHub integration, and Git providers other than GitHub.
- Native clients (macOS, Windows, Linux).
- Uploading binary files into skills.
- Choosing the event bus technology (still an open decision in the constitution).

## Impact

- `core/migrations/`: users gain `kind` (`human`/`bot`) with a nullable password for bots; bot client credentials; delegated sessions. New endpoints under `/api/core/v1/bots` and `/api/core/v1/delegated-sessions`; OAuth token endpoint gains the `client_credentials` grant for bot members. Core version bumps to a minor release (additive contract, rule 61).
- New `modules/genagents/` service with its own schema, role, migrations, `Dockerfile`, and `module.yaml`; uses the module encryption key the installer already provisions (`GENAGENTS_ENCRYPTION_KEY`).
- New `runner/` Go component (`genwork-runner`) built and published with every release; constitution component table gains its row.
- `modules/gentasks/`: assignee validation accepts bot members; a client for the genagents v1 API; the bot review endpoint; additive board view fields.
- `mcp/`: bot-member tokens are accepted like user tokens; new genmonitor and genagents toolsets so the admin chat reaches every enabled component.
- `modules/gentasks/` board GitHub settings gain `repository` (`owner/name`), the repository bots work in.
- `web/src/app/genagents/` (new) and `web/src/app/gentasks/` (assignee picker, conversation panel, "Bot review"), English and pt-BR copy.
- No new Go or npm dependencies expected beyond what the services already use; the runner shells out to the provider CLIs installed on its machine.
