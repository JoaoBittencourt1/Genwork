## Why

Teams already pay for several AI assistants (Claude, ChatGPT/Codex, Cursor, Grok), and each person uses them in isolation, outside the place where work is planned. Genwork already holds the work (tasks, boards, pull requests, infrastructure) and an MCP server that exposes it. The opportunity is to make Genwork the place where AI agents are **connected, configured, given work, coordinated, and held to goals**: an AI orchestrator inspired by Paperclip (org chart of agents, ticket checkout, heartbeats, budgets, governance) and OpenBot (a manager agent routing work to specialized agents in a shared workspace), with goals and cycles expressed as **OKRs**.

This change is the **umbrella**: it describes the whole orchestrator, fixes the architecture every delivery follows, and orders the deliveries. It is not implemented directly; each delivery is its own OpenSpec change.

## What Changes

The orchestrator lives in a new module, **`genagents`**, plus generic extension points in core. The full capability set:

1. **AI accounts.** An admin links any number of AI accounts per provider (Anthropic, OpenAI, Cursor, xAI, more later), by **subscription plan** (signed in on a runner through the provider's official CLI) or **API key**. Two Claude accounts side by side are a supported case, and each bot picks which account and model it uses. *(Delivery 1)*
2. **Bots.** Configured AI workers with a kind (`dev`, `reviewer`, `assistant`, and later `manager`), an account and model, parameters, instructions, **skills**, and **MCP servers**. Each bot is a **core bot member**: it has a role, appears wherever members appear, and acts through the Genwork MCP as itself. *(Delivery 1)*
3. **Skills** in the Agent Skills format, authored or imported in Genwork and installed into every run of the bots they are attached to. Later: versions with rollback and a test bench against saved inputs. *(Delivery 1; versioning in Delivery 4)*
4. **Runners** (`genwork-runner`) that execute the provider CLIs on machines the admin controls, inside a **disposable sandbox container per run**, holding subscription sign-ins, claiming runs atomically, and streaming progress. *(Delivery 1; a sandbox egress allowlist in Delivery 4)*
5. **Task runs.** Assigning a task to a dev bot makes it work the task end to end and open a pull request, while a **conversation panel** on the task shows its status and lets people talk to it. *(Delivery 1)*
6. **Bot review.** A "Bot review" button on tasks makes a reviewer bot review the linked pull requests and comment on GitHub. *(Delivery 1)*
7. **Admin chat.** An admin-only chat with an AI that acts as the admin across all of Genwork. *(Delivery 1)*
8. **Orchestration.** **Manager bots** and an **org chart** (every bot reports to a manager bot or to a human). Managers break objectives and large tasks into subtasks, delegate them to bots that fit (by kind, skills, and load), follow up, and escalate to humans. Work arrives through **heartbeats**: scheduled wake-ups, assignments, mentions, comments, and approval outcomes, coalesced in a wake-up queue. **Atomic checkout** guarantees one bot per task at a time. **Routines** (cron or webhook) create tracked tasks and wake their bot. *(Delivery 2)*
9. **OKRs, goals, and cycles.** **Cycles** (for example quarters, with planned/active/closed states), **Objectives** (owned by a human or a bot, optionally nested under a parent objective), and **Key Results** (numeric, percentage, boolean, or task-completion metrics with start, target, and current values and a confidence level). Tasks and boards link to key results, so every bot run carries its **goal ancestry** (KR → objective → parent objective) as context, and bots post **check-ins** on the key results they move. Task-completion KRs update themselves from linked tasks. Cycle close produces a scored retrospective. *(Delivery 3)*
10. **Budgets and cost tracking.** Usage and cost are recorded for every run (tokens and cost from API-key runs; turns and minutes for subscription runs, whose cost is the plan) and aggregated by bot, account, provider, model, task, objective, and cycle. Monthly budgets per installation, per account, and per bot warn at 80% and **pause** at 100% until an admin raises the budget or overrides once. *(Delivery 4)*
11. **Governance.** **Approvals**: configurable gates where a run pauses until a human approves, rejects, or asks for a revision (for example before a dev bot pushes to a protected branch, before a manager hires (creates) a bot, or on a budget override). An **audit log** of every bot action, configuration change (with revisions and rollback for bot configuration), approval, and cost event. Bots can be paused, resumed, and terminated. *(Delivery 4)*
12. **Provider API runtime.** Chat, planning, and review runs can execute directly against provider APIs from the genagents container when the account uses an API key, so they need no runner. Coding runs keep using runners. *(Delivery 5)*
13. **Events.** Once the platform chooses its event bus (an open decision in the constitution), gentasks ↔ genagents notifications and reconciliation move to versioned events. *(Delivery 6, blocked on that decision)*

Components touched across all deliveries: **core** (bot members, delegated sessions), **genagents module** (new), **gentasks module** (bot assignees, review request, KR links), **runner** (new component), **mcp** (toolsets), **cli** (installer, release), **web**. Native clients are out of scope for every delivery listed here. Each delivery restates its components and justifies any cross-component span (rule 63).

### `modules/genagents/module.yaml` (end state)

- `id: genagents`, `core: ">=1.4.0 <2.0.0"`, `apiPrefix: /api/genagents/v1`, `requires: []` (gentasks is an optional integration, rule 55)
- Permissions from Delivery 1 (`genagents.account.manage`, `genagents.bot.read`, `genagents.bot.manage`, `genagents.skill.manage`, `genagents.mcpserver.manage`, `genagents.runner.manage`, `genagents.run.read`, `genagents.run.write`, `genagents.adminchat.use`) plus, in later deliveries: `genagents.orgchart.manage` [admin], `genagents.routine.manage` [admin], `genagents.okr.read` [admin, member], `genagents.okr.write` [admin, member], `genagents.cycle.manage` [admin], `genagents.budget.read` [admin], `genagents.budget.manage` [admin], `genagents.approval.decide` [admin], `genagents.audit.read` [admin]
- Events: none until the event bus decision; then it publishes run and approval events and consumes task assignment events (Delivery 6)

## Capabilities

Delivery 1 is specified in its own change, **`genagents-bots-foundation`** (capabilities `core-bot-members`, `core-delegated-sessions`, `genagents-ai-accounts`, `genagents-bots`, `genagents-skills`, `genagents-mcp-servers`, `genagents-runners`, `genagents-task-runs`, `genagents-bot-reviews`, `genagents-admin-chat`, `genagents-web-ui`, `gentasks-bot-assignment`). They are not repeated here, so archiving both changes never produces duplicate requirements. This change specifies the capabilities of Deliveries 2–5 at the level needed to keep Delivery 1's design compatible. Each later delivery refines them in its own change.

### New Capabilities

- `genagents-orchestration`: manager bots, the org chart, delegation, heartbeats and the wake-up queue, atomic checkout, and routines.
- `genagents-okrs`: cycles, objectives, key results, check-ins, task and board links, goal ancestry in runs, and cycle retrospectives.
- `genagents-budgets`: usage and cost records, aggregation, and budgets that warn and pause.
- `genagents-governance`: approvals, the audit log, configuration revisions with rollback, and bot pause, resume, and terminate.
- `genagents-api-runtime`: running chat, planning, and review runs directly against provider APIs without a runner.

### Modified Capabilities

None in `openspec/specs/`.

## Non-goals

- Native clients (macOS, Windows, Linux) for any orchestrator feature.
- A marketplace of bots, skills, or company templates.
- Multiple isolated "companies" per installation: one installation is one organization.
- Training or fine-tuning models, or hosting models locally (a local model can be added later as a provider through a runner adapter).
- Bots merging pull requests or deploying without an approval gate.
- Choosing the event bus in this change: Delivery 6 waits for that decision.

## Impact

- Introduces the `genagents` module and the `runner/` component (Delivery 1), then grows genagents with orchestration, OKR, budget, and governance tables and APIs (Deliveries 2–4).
- gentasks gains links from tasks and boards to key results (Delivery 3) and keeps the optional-integration behavior: everything bot- or OKR-related is hidden when genagents is disabled.
- The MCP server gains orchestration, OKR, and approval tools as each delivery lands, so the admin chat and external assistants can drive the orchestrator.
- Provider SDK dependencies arrive only with Delivery 5, each justified in that change (rule 77).
