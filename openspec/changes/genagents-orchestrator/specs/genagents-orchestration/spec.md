## Purpose

Coordinates bots as a team: manager bots and an org chart, delegation of work, heartbeats that wake bots, exclusive task checkout, and routines that create recurring work.

## ADDED Requirements

### Requirement: Org chart
Every bot SHALL report to exactly one manager: a bot of kind `manager` or a human member. A caller holding `genagents.orgchart.manage` SHALL set and change reporting lines; a change that would create a cycle SHALL be rejected with 400. The org chart SHALL be readable as a tree by callers holding `genagents.bot.read`.

#### Scenario: Cycle rejected
- **WHEN** an admin sets manager bot A to report to bot B, which reports to A
- **THEN** the request is rejected with 400

#### Scenario: Org chart view
- **WHEN** a member opens the org chart
- **THEN** humans and bots are shown as a tree of reporting lines with each bot's kind and status

### Requirement: Manager bots delegate
A `manager` bot SHALL be able, through the Genwork MCP, to create subtasks of a task or an objective, assign them to bots that report to it (directly or indirectly) and fit by kind and skills, follow their progress, and escalate to its own manager. A manager SHALL NOT assign work to bots outside its reporting subtree, and SHALL NOT create bots unless an approval allows it.

#### Scenario: Break down a task
- **WHEN** a manager bot is assigned a large task
- **THEN** it creates subtasks linked to the parent, assigns them to dev bots in its subtree, and posts the plan in the parent task's conversation

#### Scenario: Outside the subtree
- **WHEN** a manager tries to assign a task to a bot that reports to another manager
- **THEN** the assignment is refused with a reason the manager can read

### Requirement: Heartbeats and wake-up queue
A bot SHALL work only during heartbeats. A heartbeat SHALL be triggered by: its schedule (a per-bot interval or cron expression), a task assignment, a new comment or message on a task it holds, a mention, an approval outcome, or a manual "Run now". Triggers for the same bot SHALL be coalesced while a heartbeat is queued, and a paused or over-budget bot SHALL NOT wake.

#### Scenario: Coalesced triggers
- **WHEN** three comments arrive on a bot's task within its queue window
- **THEN** one heartbeat runs with all three in its context

#### Scenario: Scheduled check
- **WHEN** a manager bot's schedule fires every morning at 09:00
- **THEN** it wakes, reviews its subtree's open work, and posts a status update

### Requirement: Atomic checkout
A task SHALL be held by at most one bot at a time. A bot SHALL check a task out before working on it and release it when done, blocked, or reassigned. A checkout attempt on a task held by another bot SHALL fail without side effects, and a checkout held by a run that ended abnormally SHALL be released.

#### Scenario: Two bots race
- **WHEN** two bots try to check out the same task at the same moment
- **THEN** exactly one succeeds

### Requirement: Routines
A caller holding `genagents.routine.manage` SHALL define routines with a trigger (cron expression or webhook with a secret), a task template (board, title, description, labels), and a bot. Each trigger SHALL create a tracked task from the template and assign it to the bot. A webhook trigger with an invalid secret SHALL be rejected without creating anything.

#### Scenario: Weekly dependency update
- **WHEN** a routine runs every Monday with template "Update dependencies" and "Dev Bot"
- **THEN** a new task is created each Monday and assigned to "Dev Bot", which starts working it
