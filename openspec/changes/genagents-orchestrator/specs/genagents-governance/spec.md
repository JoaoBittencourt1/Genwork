## Purpose

Keeps humans in control of bots: approval gates for sensitive actions, a complete audit log, revisioned bot configuration, and pause, resume, and terminate controls.

## ADDED Requirements

### Requirement: Approval gates
genagents SHALL support approval gates configured by admins: before a dev bot pushes to a branch matching a protected pattern, before a manager bot creates a bot, before a budget override, and before a bot changes a key result's target. When a gate triggers, the run SHALL pause in `waiting_for_approval` and callers holding `genagents.approval.decide` SHALL approve, reject, or request a revision with a note. Approvals SHALL NOT expire; the decision SHALL wake the bot.

#### Scenario: Approve a bot creation
- **WHEN** a manager bot asks to create a new reviewer bot and an admin approves
- **THEN** the bot is created and the manager's heartbeat resumes with the outcome

#### Scenario: Request revision
- **WHEN** an admin requests a revision with a note
- **THEN** the bot receives the note and the run continues without performing the gated action

### Requirement: Audit log
genagents SHALL record an immutable audit entry for every configuration change, run start and end, tool call that writes, approval request and decision, budget change, and override, with actor (human or bot), time, target, and a summary. Callers holding `genagents.audit.read` SHALL filter it by actor, bot, target, and time. Secrets SHALL never appear in it.

#### Scenario: Who changed the model
- **WHEN** an admin filters the audit log by "Dev Bot"
- **THEN** the entry for the model change shows who made it, when, and the old and new model

### Requirement: Configuration revisions
Every change to a bot's configuration SHALL create a revision. An admin SHALL compare revisions and roll back to an earlier one, which creates a new revision with the old values.

#### Scenario: Roll back
- **WHEN** an admin rolls "Dev Bot" back to revision 3
- **THEN** the bot's configuration equals revision 3 and a new revision records the rollback

### Requirement: Pause, resume, terminate
An admin SHALL pause a bot (no new heartbeats, active runs finish), resume it, or terminate it (active runs are cancelled, its tasks are released and reassigned to its manager).

#### Scenario: Terminate
- **WHEN** an admin terminates a dev bot holding two tasks
- **THEN** its runs are cancelled and both tasks are reassigned to its manager with a note
