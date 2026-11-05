## Purpose

Defines how a dev bot works a gentasks task end to end once assigned, and the task conversation through which it reports status and talks with people.

## ADDED Requirements

### Requirement: Start a task run on assignment
When gentasks notifies genagents that a task was assigned to the member of an enabled `dev` bot, genagents SHALL create a queued task run for that task, recording who assigned it. A task SHALL have at most one active task run; a new assignment to the same bot while a run is active SHALL NOT start another. Unassigning the bot, or assigning the task to someone else, SHALL cancel the active run.

#### Scenario: Assign to a dev bot
- **WHEN** a member assigns `GEN-42` to "Dev Bot"
- **THEN** a task run is queued for `GEN-42` with that member as the requester, and the conversation shows "Queued"

#### Scenario: Reassigned to a human
- **WHEN** `GEN-42` is reassigned from "Dev Bot" to a person while its run is working
- **THEN** the run is cancelled and the conversation says who reassigned it

### Requirement: Task context
The task run prompt SHALL include the task key, title, description, labels, board, linked pull requests, and the repository to work in. The repository SHALL be the one configured on the task's board for the gentasks GitHub integration; a task whose board has no repository SHALL fail the run with reason `no_repository` before any runner claims it.

#### Scenario: Board without repository
- **WHEN** a task on a board without a repository is assigned to a dev bot
- **THEN** the run fails with `no_repository` and the conversation tells the requester how to set one

### Requirement: What a dev bot does
A dev bot run SHALL: create a branch named after the task key, implement the task, run the repository's checks it can find, push the branch, and open a pull request whose title cites the task key, so the existing GitHub sync links and moves the task. It SHALL post status updates in the conversation as it goes and SHALL end with a summary and the pull request link. It SHALL NOT merge pull requests.

#### Scenario: Successful run
- **WHEN** "Dev Bot" finishes `GEN-42`
- **THEN** a pull request titled with `GEN-42` exists, the task shows it, and the conversation ends with the summary and link

#### Scenario: Bot needs input
- **WHEN** the bot cannot proceed without a decision
- **THEN** it asks in the conversation, the run status becomes `waiting_for_input`, and the run resumes when someone replies

### Requirement: Run states
A run SHALL be in one state at a time: `queued`, `running`, `waiting_for_input`, `succeeded`, `failed` (with a reason), or `cancelled`. Only `queued`, `running`, and `waiting_for_input` are active. A `waiting_for_input` run with no reply in 24 hours SHALL fail with reason `no_reply`.

#### Scenario: Cancel from the conversation
- **WHEN** the requester presses "Stop" on a running run
- **THEN** the runner stops the CLI and the run becomes `cancelled`

### Requirement: Task conversation
Each task SHALL have one conversation holding its runs' events and people's messages in order. A caller holding `genagents.run.read` and `gentasks.board.read` SHALL read it; a caller holding `genagents.run.write` SHALL post messages. Messages SHALL be plain text up to 8000 characters. Reads SHALL support fetching only entries after a given sequence so clients can poll cheaply.

#### Scenario: Requester talks to the bot
- **WHEN** the requester posts "also add a test for the empty case"
- **THEN** the message appears in the conversation with their name and is delivered to the bot

#### Scenario: Incremental read
- **WHEN** a client reads the conversation after sequence 40
- **THEN** only entries with a sequence above 40 are returned
