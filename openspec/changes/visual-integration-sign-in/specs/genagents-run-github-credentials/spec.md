## Purpose

Gives each run that clones or posts on GitHub a short-lived token from the platform's GitHub App, so runners need no GitHub token in their environment.

## ADDED Requirements

### Requirement: Token relay
A runner SHALL obtain a GitHub token for an active task or review run it holds with `POST /api/genagents/v1/runner/runs/{token}/github-token`. Genagents SHALL request it from gentasks with the bot's own Genwork token, for the run's repositories. When gentasks cannot issue one, genagents SHALL answer 409 `github_token_unavailable` with a `reason`.

#### Scenario: Task run
- **WHEN** the runner holding a task run on `acme/api` asks for a GitHub token
- **THEN** it receives a token for `acme/api` and its expiry, with `Cache-Control: no-store`

#### Scenario: Chat run
- **WHEN** a runner asks for a GitHub token for a chat run
- **THEN** genagents answers 409 `github_token_unavailable`

### Requirement: Runner use and fallback
The runner SHALL use the platform token for git on the host and for GitHub REST calls, renew it before it expires, and fall back to its own `GH_TOKEN`. Without either, a run that needs GitHub SHALL fail with `github_credentials_missing`. The token SHALL never enter the sandbox.

#### Scenario: Runner without GH_TOKEN
- **WHEN** a runner without `GH_TOKEN` claims a task run and gentasks issues a token
- **THEN** the clone, push, and pull request use that token

### Requirement: Eligibility
A runner without its own GitHub credentials SHALL be eligible for runs that need GitHub.

#### Scenario: Claim
- **WHEN** a task run is queued and the only runner reports no GitHub credentials
- **THEN** the runner claims it
