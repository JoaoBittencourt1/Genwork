## Purpose

Records what every bot run consumes and enforces monthly budgets, so AI spending is visible and bounded.

## ADDED Requirements

### Requirement: Usage records
Every run SHALL record its usage when it ends: provider, account, model, input and output tokens and cost when the provider reports them (API-key runs), and turns and wall-clock minutes for every run. Subscription runs SHALL record zero monetary cost and be marked as plan-covered.

#### Scenario: API-key run
- **WHEN** an API-key run ends after using 120k input and 8k output tokens
- **THEN** its usage record holds those tokens and the cost the provider reported or the catalog price computes

### Requirement: Aggregation
A caller holding `genagents.budget.read` SHALL read usage aggregated by month and by bot, account, provider, model, task, objective, and cycle.

#### Scenario: Cost per objective
- **WHEN** an admin views usage for objective "Ship self-serve onboarding"
- **THEN** the total sums runs on tasks linked to its key results

### Requirement: Budgets
A caller holding `genagents.budget.manage` SHALL set monthly budgets in currency for the installation, for each account, and for each bot. At 80% of any applicable budget, admins SHALL see a warning; at 100%, affected bots SHALL be paused: queued runs wait and no heartbeat fires until an admin raises the budget, a new month starts, or an admin approves a one-time override.

#### Scenario: Bot hits its budget
- **WHEN** "Dev Bot" reaches 100% of its monthly budget during a run
- **THEN** the current run finishes, no new run of "Dev Bot" starts, and admins see why

#### Scenario: Override
- **WHEN** an admin approves a one-time override for "Dev Bot"
- **THEN** its queued runs start and the override is recorded in the audit log
