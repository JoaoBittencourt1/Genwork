## Purpose

Defines the runner: a process on a machine the admin controls that holds subscription sign-ins, claims runs from genagents, executes the provider CLIs in isolated workspaces, and reports progress back.

## ADDED Requirements

### Requirement: Runner registration
A caller holding `genagents.runner.manage` SHALL be able to create a runner with a name; genagents SHALL return a registration token once, valid for one hour and single use. `genwork-runner register` with the instance address and that token SHALL exchange it for a long-lived runner secret, stored by the runner on its machine; genagents SHALL keep only its hash (rule 67). An admin SHALL be able to revoke a runner, which rejects its secret immediately and fails its running runs.

#### Scenario: Register
- **WHEN** an admin creates runner "build-box" and runs the shown register command on that machine within an hour
- **THEN** the runner appears as online

#### Scenario: Reused registration token
- **WHEN** a registration token is used a second time
- **THEN** the exchange fails

#### Scenario: Revoked runner
- **WHEN** an admin revokes "build-box"
- **THEN** its next request is rejected and its running runs fail with reason `runner_revoked`

### Requirement: Runner presence and tool report
A runner SHALL send a heartbeat at least every 30 seconds with the provider CLIs it found and their versions. genagents SHALL show a runner as `online` when its last heartbeat is under 90 seconds old and `offline` otherwise. Accounts SHALL only be linked to runners that report their provider's CLI.

#### Scenario: Runner goes away
- **WHEN** a runner stops sending heartbeats for 90 seconds
- **THEN** it shows as offline

### Requirement: Subscription sign-in on the runner
For each subscription account, the runner SHALL keep a separate sign-in profile so two accounts of the same provider never share credentials. `genwork-runner account login <account>` SHALL start the provider CLI's own sign-in inside that account's profile and report the result to genagents. Credentials SHALL stay on the runner machine and SHALL NOT be sent to genagents.

#### Scenario: Two Claude subscriptions on one runner
- **WHEN** an admin signs in "Claude — João" and "Claude — Team" on the same runner
- **THEN** each run uses only its own account's profile

### Requirement: GitHub credentials on the runner
A runner SHALL be configured on its machine with GitHub credentials (a fine-grained token with contents and pull request access to the repositories bots work in), kept only on that machine. Runs SHALL clone, push, open pull requests, and post reviews with them. A runner without GitHub credentials SHALL report it in its heartbeat and SHALL NOT claim task runs or reviews; it SHALL still be eligible for admin chat runs.

#### Scenario: Runner without GitHub credentials
- **WHEN** a runner reports no GitHub credentials and a task run is queued
- **THEN** that runner does not receive the task run, and the Runners page shows the missing credentials

### Requirement: Atomic run claiming
A runner SHALL ask genagents for work and receive at most one queued run at a time per free slot (default 2 concurrent runs, configurable on the runner). A run SHALL be claimable only by a runner that holds its account (subscription) or by any online runner reporting the provider's CLI (API key). Two runners SHALL never claim the same run. A claimed run whose runner stops sending heartbeats for 5 minutes SHALL fail with reason `runner_lost`.

#### Scenario: Concurrent claims
- **WHEN** two runners ask for work while one run is queued
- **THEN** exactly one of them receives it

#### Scenario: Runner lost mid-run
- **WHEN** the runner holding a run is offline for 5 minutes
- **THEN** the run fails with `runner_lost` and the conversation says so

### Requirement: Run execution
On claim, the runner SHALL receive the run specification: kind, prompt, instructions, model, parameters, skills, MCP servers, the API key for `api_key` accounts, and for task runs and reviews the repository and branch. It SHALL create a fresh workspace per run, check out the repository when there is one, configure the CLI for skills and MCP servers, run the CLI headless, and delete the workspace and every secret file when the run ends. The run timeout SHALL be enforced by the runner and by genagents.

#### Scenario: Timeout
- **WHEN** a run exceeds its bot's run timeout
- **THEN** the runner stops the CLI and the run fails with reason `timeout`

#### Scenario: Workspace cleanup
- **WHEN** a run ends in any state
- **THEN** its workspace and any written key or MCP configuration file are deleted

### Requirement: Progress reporting
While a run executes, the runner SHALL post run events to genagents in order: status changes, assistant messages, short tool-use summaries (tool name and a one-line description, never secrets), and a final result. Events SHALL carry a per-run sequence number, and genagents SHALL ignore duplicates so a retried post never appears twice.

#### Scenario: Retried post
- **WHEN** the runner re-posts event 12 after a network timeout
- **THEN** the conversation shows event 12 once

### Requirement: Messages during a run
The runner SHALL fetch new human messages for its runs and deliver them to the model at the next turn boundary by resuming the CLI session with the message. A message sent after the run finished SHALL start a follow-up run on the same session when the bot and account still exist.

#### Scenario: Message while working
- **WHEN** the person who assigned the task writes "use the existing TaskRepository" while the bot is editing files
- **THEN** the bot receives the message when its current turn ends and the reply appears in the conversation
