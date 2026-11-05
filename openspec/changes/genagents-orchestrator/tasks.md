### Phased delivery

This umbrella change is delivered as separate OpenSpec changes, one per delivery. Each delivery has its own proposal, specs, design, and phased `tasks.md` with one branch and one PR per phase (rules 42–48). The phases below are the deliveries. Each one is done when its change is merged and archived.

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | Delivery 1, `genagents-bots-foundation`: AI accounts, bots, skills, MCP servers, runners, task runs and conversation, bot review, admin chat, core bot members and delegated sessions | — | — |
| 2 | Delivery 2, orchestration: manager bots, org chart, delegation, heartbeats and wake-up queue, atomic checkout, routines | 1 | 3 |
| 3 | Delivery 3, OKRs: cycles, objectives, key results, check-ins, task and board links, goal ancestry, retrospectives | 1 | 2 |
| 4 | Delivery 4, budgets and governance: usage records, budgets, approvals, audit log, configuration revisions, pause, resume, terminate; skill versions | 2 | 5 |
| 5 | Delivery 5, provider API runtime for chat, review, and heartbeat runs on API-key accounts | 1 | 2, 3, 4 |
| 6 | Delivery 6, events: move gentasks ↔ genagents hints and reconciliation to the platform event bus | event bus decision, 2 | — |

## Phase 1 — Delivery 1 (change: genagents-bots-foundation)

- [ ] 1.1 Implement every phase of `genagents-bots-foundation/tasks.md`
- [ ] 1.2 Manual verification with the developer and archive `genagents-bots-foundation`

## Phase 2 — Delivery 2: orchestration (change: genagents-orchestration)

- [ ] 2.1 Propose `genagents-orchestration` with `/opsx:propose`, refining the `genagents-orchestration` spec of this change
- [ ] 2.2 Implement its phases (wake-up queue and checkout in genagents, manager bot kind, org chart API and page, delegation tools in the MCP, routines)
- [ ] 2.3 Manual verification and archive

## Phase 3 — Delivery 3: OKRs (change: genagents-okrs)

- [ ] 3.1 Propose `genagents-okrs`, refining the `genagents-okrs` spec of this change
- [ ] 3.2 Implement its phases (cycles, objectives, key results, check-ins, links from gentasks tasks and boards, goal ancestry in run prompts, OKR pages, OKR tools in the MCP, retrospective)
- [ ] 3.3 Manual verification and archive

## Phase 4 — Delivery 4: budgets and governance (change: genagents-budgets-governance)

- [ ] 4.1 Propose `genagents-budgets-governance`, refining the `genagents-budgets` and `genagents-governance` specs of this change, plus skill versions and test bench
- [ ] 4.2 Implement its phases (usage records from runner and API results, budgets enforced at claim and wake-up, approval gates, audit log, bot revisions and rollback, pause, resume, terminate)
- [ ] 4.3 Manual verification and archive

## Phase 5 — Delivery 5: API runtime (change: genagents-api-runtime)

- [ ] 5.1 Propose `genagents-api-runtime`, refining the `genagents-api-runtime` spec of this change, and justify each provider SDK dependency (rule 77)
- [ ] 5.2 Implement its phases (execution contract inside genagents, tool loop over the Genwork MCP and HTTP MCP servers, per-provider API clients with recorded fixtures, `execution` field)
- [ ] 5.3 Manual verification and archive

## Phase 6 — Delivery 6: events (change: genagents-events)

- [ ] 6.1 Raise the event bus decision with the developer (constitution open decision) and record it in the constitution
- [ ] 6.2 Propose `genagents-events`: event contracts in `module.yaml` for gentasks and genagents, publisher and consumer in `sdk/go/`
- [ ] 6.3 Implement its phases, keeping reconciliation as a safety net
- [ ] 6.4 Manual verification and archive

## Phase 7 — close the umbrella

- [ ] 7.1 Archive `genagents-orchestrator` once Deliveries 2–5 have archived their refined versions of its specs
