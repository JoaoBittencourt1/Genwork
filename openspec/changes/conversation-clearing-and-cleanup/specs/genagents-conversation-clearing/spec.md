## Purpose

Lets people start a task's bot conversation over, removing its history from the database.

## ADDED Requirements

### Requirement: Clear a task conversation
A caller holding `genagents.run.write` who can read the task SHALL clear its bot conversation with `DELETE /api/genagents/v1/tasks/{taskToken}/conversation`. Clearing SHALL cancel the active run, if any, and physically delete the conversation's entries and ended runs, and the conversation itself when no run is left.

#### Scenario: Ended conversation
- **WHEN** a member clears a task whose bot runs all ended
- **THEN** the conversation reads empty and no entry or run of it remains in the database

#### Scenario: Active run
- **WHEN** a member clears a task while its bot is running
- **THEN** the run is cancelled, its entries are deleted, and the runner stops on its next poll

#### Scenario: No permission
- **WHEN** a caller without `genagents.run.write` clears a conversation
- **THEN** the request fails with 403 and nothing is deleted
