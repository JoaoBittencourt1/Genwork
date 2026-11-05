## Context

See proposal.md for motivation. The detailed design of Delivery 1 (bot members, delegated sessions, the runner protocol, provider adapters, the data model, and authorization) lives in `genagents-bots-foundation/design.md`. This document fixes the cross-delivery architecture so every later delivery extends Delivery 1 instead of reworking it.

References studied:
- **Paperclip** (paperclipai/paperclip): companies with an org chart of agents reporting to a manager, issues with atomic checkout, heartbeats from a coalescing wake-up queue (schedule, assignment, comment, approval, manual), runtime adapters (`claude_local`, `codex_local`, `cursor`, `gemini_local`, …) exposing `execute`, `testEnvironment`, and a session codec, per-agent and per-company monthly budgets (warn at 80%, pause at 100%), approvals (hire, strategy, budget override, task review), an activity log, revisioned configuration, and company skills. It tracks goal ancestry on tasks, but has no OKRs or cycles.
- **OpenBot** (meetopenbot/openbot): a local-first runtime with a Manager Agent that reads intent, keeps long-term memory, and routes work to specialized workers (OS, browser, Codex) over an event bus, with channels and tagging to address agents.

Genwork's take: Paperclip's governance model (org chart, checkout, heartbeats, budgets, approvals) on top of Genwork's existing work objects (gentasks tasks, boards, pull requests), OpenBot's manager-routes-to-specialists pattern for manager bots, and OKRs in place of Paperclip's free-form goals.

## Goals / Non-Goals

**Goals:** one architecture that holds from Delivery 1 to the end state: a single module, one identity model, one run model, one conversation model, and one execution contract for runners and the API runtime.

**Non-Goals:** detailing Deliveries 2–6 down to tables and endpoints; each delivery's change does that.

## Decisions

### 1. One module, `genagents`, for the whole orchestrator
Accounts, bots, runs, orchestration, OKRs, budgets, and governance share the run and bot models, and splitting them would force chatty cross-module APIs over the same data (rules 53–54). OKRs were considered as a separate `gengoals` module usable without bots. That was rejected for now: the developer framed OKRs as the orchestrator's goals and cycles, and their main consumer is goal ancestry in runs. If humans-only OKRs become a need, the OKR feature folder (`internal/okrs`) is self-contained and can be extracted behind the same `/api/genagents/v1/okrs` contract (rule 61).

### 2. Identity: bots are core bot members
Delivery 1 introduces bot members authenticated with OAuth `client_credentials`. Every later feature attributes actions to the bot member token: audit entries, check-ins, subtask creation, and assignments. Manager bots get no special core power; their delegation limits (their own subtree) are enforced by genagents, and the core role bounds what any bot can touch.

### 3. One run model for every kind of work
Delivery 1 defines runs (`task`, `review`, `chat`) with states, conversations, and sequenced entries. Later deliveries add run kinds (`heartbeat`, `routine`) and states (`waiting_for_approval`) to the same tables and the same runner and API-runtime contract, so clients render every run the same way. Heartbeats are runs whose prompt is the coalesced trigger set.

### 4. Execution contract: runner first, API runtime later
Runners (Delivery 1) handle everything, including subscription plans that only work through CLIs. The API runtime (Delivery 5) implements the same execution contract inside genagents: claim → spec → events → result. Run states, entries, and usage stay identical (spec `genagents-api-runtime`). This mirrors Paperclip's adapters: the orchestrator never knows how a run executes.

### 5. Wake-up queue and checkout
Delivery 2 adds a `wakeups` table (bot, trigger type, payload, `coalesce_until`), drained into heartbeat runs, with one queued heartbeat per bot. Checkout is a row in `task_checkouts` with a partial unique index on the active task (`SELECT … FOR UPDATE SKIP LOCKED` on claim), released when a run ends, by the sweeper on `runner_lost`, or on reassignment. Delivery 1's rule "one active task run per task" is the first form of checkout and is replaced by it without changing the API.

### 6. OKR model
Delivery 3 adds:
- `cycles`: name, cadence, `starts_on`, `ends_on`, state, plus an exclusion constraint against overlapping ranges per cadence
- `objectives`: cycle, owner token, `parent_objective_id`, frozen score
- `key_results`: metric type, start, target, current, confidence
- `key_result_checkins`
- `key_result_links`: target kind `task` or `board`, gentasks token

Task-completion progress is computed from gentasks task states, read through gentasks' public API in batch (rule 21). It is cached per KR and refreshed on run end and every 5 minutes. Goal ancestry is a prompt section composed by genagents, so runners need no change.

### 7. Budgets and usage
Delivery 4 adds `usage_records` (one per run) and `budgets` (scope: installation, account, or bot; monthly amount; currency). Enforcement happens at claim and wake-up time: an over-budget bot's runs are not eligible, which reuses Delivery 1's claim eligibility. Prices for API-key runs come from the provider's reported cost when present, and otherwise from a price table in the provider catalog, which stays the single owner of provider facts (rule 7).

### 8. Governance
Delivery 4 also adds `approval_requests`, `audit_entries` (append-only, with no update or delete path, and an index on actor, target, and time), and `bot_revisions` (a full configuration snapshot per change). Gates are checked in genagents before it performs a gated action on a bot's behalf. Gates on Git pushes are enforced by the runner asking genagents before pushing a protected branch.

### 9. Events after the bus decision
Deliveries 1–5 use versioned HTTP calls plus reconciliation (Delivery 1 design §9). The constitution keeps the event bus open, so Delivery 6 waits for the developer's choice. The contracts it will carry are already known: `gentasks.task.assigned.v1`, `gentasks.task.commented.v1`, `genagents.run.ended.v1`, and `genagents.approval.decided.v1`.

### 10. Simple UI first
Every delivery ships functional, minimal screens from the shared shadcn components (Delivery 1 design §13). Visual design is a later, separate change once the flows settle.

## Risks / Trade-offs

- [Scope is large] → Six deliveries, each an independent change with its own phases and PRs; Delivery 1 alone is usable.
- [Autonomous managers can create runaway work] → Subtree-only delegation, approval gates on bot creation, budgets that pause, coalesced heartbeats, and the audit log.
- [Subscription-plan terms] → API-key mode always available; the admin chooses; documented in Delivery 1.
- [OKRs inside genagents limit human-only use] → Self-contained feature folder with a stable contract; extraction path noted in decision 1.
- [Event bus undecided] → Reconciliation keeps correctness without events; Delivery 6 is blocked only on the decision, not on code.

## Migration Plan

Each delivery migrates additively within the `genagents` schema and keeps the `v1` contract backward compatible (rule 61). Disabling genagents at any point leaves core and gentasks working (rule 56).

## Open Questions

- Default heartbeat interval for manager bots and the coalescing window: tuned in Delivery 2 without spec changes.
- Currency for budgets: one installation-wide currency setting, chosen in Delivery 4.
