## Purpose

Expresses goals and cycles as OKRs (objectives with measurable key results inside time-boxed cycles), links work to them, and gives every bot run the "why" behind its task.

## ADDED Requirements

### Requirement: Cycles
A caller holding `genagents.cycle.manage` SHALL create cycles with a name, start date, and end date; cycles of the same cadence SHALL NOT overlap. A cycle SHALL be `planned`, `active`, or `closed`; at most one cycle SHALL be active at a time, and only a `planned` cycle can be activated. Closing a cycle SHALL freeze its objectives' scores.

#### Scenario: Activate a quarter
- **WHEN** an admin activates the `planned` cycle "2027 Q1" while no cycle is active
- **THEN** "2027 Q1" becomes active and new objectives default to it

#### Scenario: Overlap
- **WHEN** an admin creates a quarterly cycle overlapping an existing quarterly cycle
- **THEN** the request is rejected with 400

### Requirement: Objectives
A caller holding `genagents.okr.write` SHALL create objectives with a title, description, cycle, owner (a human or a bot member), and optional parent objective (in the same or an earlier cycle). An objective's progress SHALL be the average of its key results' progress, and its score at cycle close SHALL be that progress frozen.

#### Scenario: Nested objective
- **WHEN** a team objective is created under the company objective "Ship self-serve onboarding"
- **THEN** the team objective shows its parent and contributes to the parent's roll-up view

### Requirement: Key results
An objective SHALL have one to five key results. A key result SHALL have a title, a metric type (`number`, `percentage`, `boolean`, or `task_completion`), start, target, and current values (except `task_completion`, whose target is the set of linked tasks), and a confidence level (`on_track`, `at_risk`, `off_track`). Progress SHALL be `(current − start) / (target − start)` clamped to 0–100% for numeric types, 0 or 100% for `boolean`, and the share of linked tasks done for `task_completion`.

#### Scenario: Task-completion KR
- **WHEN** a `task_completion` key result has 4 linked tasks and 3 reach a final column
- **THEN** its progress is 75% without anyone updating it

#### Scenario: Decreasing metric
- **WHEN** a `number` KR has start 400 ms, target 200 ms, and current 300 ms
- **THEN** its progress is 50%

### Requirement: Check-ins
Owners and bots working on linked tasks SHALL post check-ins on a key result: a new current value (when applicable), a confidence level, and a note. Check-ins SHALL form the key result's history, and the latest SHALL set its current value and confidence.

#### Scenario: Bot check-in
- **WHEN** a dev bot finishes a task linked to "p95 latency under 200 ms" and measures 260 ms
- **THEN** it posts a check-in with current 260, confidence `at_risk`, and a note linking its pull request

### Requirement: Links from work to key results
A task or a board SHALL be linkable to key results by callers holding `genagents.okr.write` and `gentasks.task.write`. When genagents is disabled, gentasks SHALL hide these links without errors (rule 55).

#### Scenario: Link a task
- **WHEN** a member links `GEN-42` to a key result
- **THEN** the task dialog shows the key result and objective, and the KR lists `GEN-42`

### Requirement: Goal ancestry in runs
Every run on a task linked, directly or through its board, to key results SHALL include in its context each key result with its progress and its objective chain up to the root, so the bot knows why the task matters.

#### Scenario: Context of a dev run
- **WHEN** "Dev Bot" starts `GEN-42`, which is linked to a KR under "Ship self-serve onboarding"
- **THEN** its prompt contains the KR, its progress, and the objective chain

### Requirement: Cycle retrospective
Closing a cycle SHALL produce a retrospective listing each objective's score, its key results' final values and confidence history, and the linked tasks done and not done. An admin SHALL be able to ask a manager bot to draft the narrative, which a human edits before publishing.

#### Scenario: Close the quarter
- **WHEN** an admin closes "2027 Q1"
- **THEN** scores freeze and a retrospective is available with every objective's score
