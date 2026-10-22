## Why

Teams that use gentasks still move tasks by hand when the code work behind them progresses on GitHub, so boards drift from reality. Genwork is self-hosted, so the integration has to be as close to a single "Connect" click as possible: no GitHub settings to copy, no `.env` to edit, nothing to configure per repository.

## What Changes

- New `internal/githubsync` feature in the gentasks module that moves tasks from GitHub pull request events:
  - a PR whose title or head branch cites a task key (`PREFIX-123`, case-insensitive) is opened, reopened, or edited → the task moves to the board's configured "PR opened" column;
  - a review is requested on such a PR → the task moves to the configured "review requested" column;
  - such a PR is merged into one of the board's final branches (default `main`, `master`) → the task moves to the configured "merged" column (default: the board's `done` preset column).
- Automation only moves tasks forward (to a column further right), never moves a task again once a merge put it in the merged column, and never blocks manual moves.
- An admin connects GitHub in three clicks through GitHub's App manifest flow: Genwork pre-fills a private GitHub App (webhook URL from `GENWORK_PUBLIC_URL`, read-only pull request and metadata permissions, `pull_request` event), GitHub hands the App's credentials back, and the admin installs it on the repositories they pick. Disconnecting removes the stored credentials.
- The App's credentials (webhook secret, private key) are stored in the gentasks database, encrypted with AES-256-GCM under a per-module key from the environment. This is an explicit exception to rules 1 and 67 approved by the developer (see design.md).
- A public webhook endpoint authenticated by the `X-Hub-Signature-256` HMAC signature (its rule 18 authorization rule).
- Per-board GitHub settings: which column each event moves to, and the list of final branches.
- New permission `gentasks.github.manage` (default role: admin) to connect and disconnect GitHub; per-board settings use the existing `gentasks.board.manage`.
- The installer CLI generates a `<MODULE>_ENCRYPTION_KEY` for every installed module, by convention, the same way it generates `<MODULE>_DB_PASSWORD`.
- Web: a GitHub integration page (connect, install, disconnect) and a GitHub section in the board settings page.

Components touched: **gentasks module** (feature, migration, `module.yaml`), **cli** (per-module encryption key), **web** (`web/src/app/gentasks/`). Core is not touched (rule 52). The change spans a module, the CLI, and the web app because the module needs a secret only the installer can provision and an admin screen to be usable at all (rule 63); the work is split in phases per component in tasks.md.

## Capabilities

### New Capabilities

- `gentasks-github-connection`: connecting, inspecting, and disconnecting the GitHub App through the manifest flow; encrypted credential storage.
- `gentasks-github-board-settings`: per-board mapping of GitHub events to columns and the list of final branches.
- `gentasks-github-task-sync`: the webhook endpoint, signature verification, task key matching, and the movement rules.
- `gentasks-github-web-ui`: the integration page and the board settings section in the web app.
- `module-encryption-key`: the installer generates and clears a per-module encryption key.

### Modified Capabilities

None — no capability has been archived into `openspec/specs/` yet.

## Non-goals

- Polling for installations GitHub cannot reach (no public URL). The App is created with the private key stored so a later change can add polling without reconnecting.
- GitHub Enterprise Server. The GitHub URLs come from the environment, so it stays possible later, but it is not tested here.
- Showing linked pull requests on the task, commenting on PRs, or any write access to GitHub.
- Matching task keys inside the PR body (too many false positives such as "see GEN-12").
- Per-branch target columns (e.g. `staging` → QA, `main` → Done): one merged column per board.
- An activity history for tasks: gentasks has none today; automated moves are attributed to Genwork in the link records only.
- Publishing events for other modules: the event bus is still an open decision.

## Impact

- `modules/gentasks/`: new `internal/githubsync/` (credential encryption included; it moves to a shared package only when a second feature needs it, rule 4), new migration, new routes under `/api/gentasks/v1/github/...` and `/api/gentasks/v1/boards/{token}/github-settings`, `module.yaml` gains `gentasks.github.manage`, config gains `GENTASKS_ENCRYPTION_KEY`, `GITHUB_WEB_URL`, `GITHUB_API_URL`.
- `deploy/.env.example`: new variables (`GITHUB_WEB_URL=https://github.com`, `GITHUB_API_URL=https://api.github.com`, empty `GENTASKS_ENCRYPTION_KEY`).
- `cli/`: `ModuleConventions` gains an encryption key variable; `EnsureModuleSecrets` / `ClearModuleSecrets` handle it.
- `web/src/app/gentasks/`: new page, callback route, settings section, datasource and domain code with tests; pt-BR and English copy.
- Operational: GitHub must reach `GENWORK_PUBLIC_URL` over HTTPS for webhooks to arrive.
- No new Go or npm dependencies expected (`crypto/aes`, `crypto/cipher`, `crypto/hkdf`, `crypto/hmac` are in the standard library).
