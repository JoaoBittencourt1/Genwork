## Context

See proposal.md for the motivation. Relevant current state:

- gentasks is a module with features under `modules/gentasks/internal/` (`boards`, `tasks`, `labels`, `boardview`), wired in `internal/app/app.go` through the SDK router, where every route declares a policy (`httpapi.Public`, `RequirePermission`) — rule 18.
- Boards have a unique `task_prefix` (`^[A-Z][A-Z0-9]{0,9}$`, unique among active boards) and tasks a per-board `number`, so `PREFIX-123` identifies one task across the installation. Boards are installation-wide (no workspace column).
- New boards get the columns `backlog`, `todo`, `done` (presets); "in progress" or "in review" exist only as custom columns. Column order is `board_columns.position`.
- `TaskService` already owns moving a task between columns (`moveToColumn`, `withLockedTask`, position shifting). There is no task activity history and moves record no actor.
- gentasks already reads `GENWORK_PUBLIC_URL`. The installer derives every per-module env variable from the module id (`cli/internal/modules/naming.go`) and fills secrets in `EnsureModuleSecrets`; `updateStackEnv` appends keys `.env.example` gained since the `.env` was created.
- Tests in gentasks live under `modules/gentasks/tests/` and run against Postgres; no external API may be called (rule 25).

## Goals / Non-Goals

**Goals:**
- Zero manual configuration beyond clicking through GitHub's screens: no secret copied by hand, no `.env` edit.
- Webhook handling that is safe to replay and cheap enough to run synchronously.
- No new Go or npm dependency.

**Non-Goals:**
- Calling the GitHub API with the App's private key (no installation tokens, no JWT): nothing in this change needs it. The key is stored only so a later polling change does not force admins to reconnect.
- Key rotation for `GENTASKS_ENCRYPTION_KEY`.

## Decisions

### 1. A feature inside gentasks, not a `github` module

`modules/gentasks/internal/githubsync/` owns everything GitHub-related. The only consumer of PR events is gentasks, the per-board configuration belongs to boards, and a separate module would need the event bus, which is still an open decision (constitution, "Open decisions"). Alternative considered: a `modules/github` module publishing `github.pull_request.*` events — deferred until a second module needs GitHub data; at that point `githubsync`'s webhook and connection parts move out and gentasks becomes a consumer.

The AES-GCM helper lives inside `githubsync` too; it moves to a shared package when a second feature needs it (rule 4).

### 2. GitHub App through the manifest flow

The manifest flow is GitHub's mechanism for exactly this case: a self-hosted product cannot ship one App for every installation because an App has one fixed webhook URL. The admin posts a pre-filled manifest, GitHub creates the App and redirects back with a one-time `code`, and `POST /app-manifests/{code}/conversions` returns the App's id, slug, owner, webhook secret, and private key — nothing is copied by hand.

- **Private App, owner chosen by the admin.** A private App can only be installed on the account that owns it, so the admin may enter an organization login and the form posts to `/organizations/{org}/settings/apps/new`. A public App was rejected: anyone could install it and send deliveries to the installation.
- **Name.** App names are unique across GitHub (max 34 characters). The manifest suggests `Genwork ` plus 6 random lowercase base32 characters; GitHub lets the admin edit it on the creation page if it still collides.
- **URLs.** `url` = `GENWORK_PUBLIC_URL`; `hook_attributes.url` = `{GENWORK_PUBLIC_URL}/api/gentasks/v1/github/webhook`; `redirect_url` = `{GENWORK_PUBLIC_URL}/tasks/integrations/github/callback`; `setup_url` = `{GENWORK_PUBLIC_URL}/tasks/integrations/github` (where GitHub sends the admin after installing).
- **Permissions and events.** `pull_requests: read`, `metadata: read`; events `pull_request`. Least privilege; `metadata` is mandatory for every App.
- **Client secret discarded.** The conversion also returns OAuth client credentials; no user-to-server flow exists, so they are not stored.

### 3. Credentials in the database, encrypted — exception to rules 1 and 67

Approved by the developer for this change. The webhook secret and private key are created by GitHub at runtime, so they cannot come from the environment (rule 1) without breaking the zero-configuration goal. They cannot be stored as a hash (rule 67): HMAC verification needs the original secret and the private key must stay usable. Rule 1's intent — no secret in source — and rule 67's intent — no readable secret in a database dump, never logged — are kept by:

- **AES-256-GCM** (standard library) with a key derived by **HKDF-SHA256** (`crypto/hkdf`) from `GENTASKS_ENCRYPTION_KEY`, info `gentasks/github-credentials/v1`. Random 96-bit nonce per value, stored as `nonce || ciphertext` in a `bytea` column.
- **Associated data** = the column name plus the row token, so a ciphertext copied into another column or row fails authentication.
- Decryption failure (changed key, tampering) surfaces as connection state `unreadable`; the webhook then rejects every delivery (`401`). The admin disconnects and connects again.
- The credential types implement `fmt.Formatter`/`slog.LogValuer` returning a redacted value, so they cannot leak through logs by accident.

Alternatives considered: plaintext columns (rejected: backups and dumps would leak the App's private key); a core secret store (does not exist, and core must not hold module data — rules 52, 53); env-only credentials (rejected: requires copying values from GitHub into `.env` and restarting).

Trade-off: the key sits in the same `.env` as the database password, so an attacker with both the host and the database gets everything; the encryption protects dumps, backups, and read-only database access.

### 4. Stateless manifest `state`

The `state` round-tripped through GitHub is `base64url(payload) "." base64url(HMAC-SHA256(payload))`, where the payload holds the user token, an expiry one hour ahead, and a random nonce, and the HMAC key is derived with HKDF info `gentasks/github-manifest-state/v1`. Verification checks the MAC in constant time, the expiry, and that the user matches the caller. Alternative: a `github_manifest_states` table — rejected because it needs cleanup and adds nothing.

### 5. Per-module encryption key provisioned by convention

`ModuleConventions` gains `EncryptionKeyVariable` (`<PREFIX>_ENCRYPTION_KEY`). `EnsureModuleSecrets` fills it with 32 random bytes as unpadded base64url when empty; `ClearModuleSecrets` empties it. Every module gets one, whether it uses it or not, so the installer stays free of module-specific code (rules 52, 64). gentasks requires the variable at startup and rejects values that do not decode to at least 32 bytes. A single installation-wide key was rejected: one leaked module would expose the others' secrets.

### 6. GitHub URLs from the environment

`GITHUB_WEB_URL` and `GITHUB_API_URL` are required at startup (rule 1). `deploy/.env.example` ships them as `https://github.com` and `https://api.github.com`, so the installer copies them into every `.env` — existing installations receive them through `updateStackEnv`. This also keeps GitHub Enterprise Server possible later. Tests point them at an `httptest` server or an unroutable address (rule 25).

### 7. Synchronous webhook handling, idempotent instead of deduplicated

A delivery is handled inside the request in one transaction: a few indexed queries, well within GitHub's 10-second timeout. There is no outbox or queue to hand work to yet.

Instead of a table of `X-GitHub-Delivery` ids, processing is idempotent: moves only go forward, a merge-moved task is locked, and moving to the current column is a no-op, so a replayed delivery (GitHub redelivery or an attacker replaying a captured request) changes nothing. This replaces the delivery-id deduplication discussed earlier: same guarantee, no table to prune.

Unknown keys, unconfigured events, and ignored actions answer `204` so GitHub does not mark deliveries as failed. Database errors answer `500`.

### 8. Matching task keys

`(?i)\b([a-z][a-z0-9]{0,9})-([0-9]{1,9})\b` over the PR title and `pull_request.head.ref`; prefixes are upper-cased and pairs deduplicated. All pairs resolve in one query (`unnest` of prefixes and numbers joined to `boards` and `tasks`, active rows only) — rule 21. The body is not searched (proposal, Non-goals).

### 9. Movement rules

For each resolved task, inside the delivery's transaction:

1. Load the board's settings (defaults when no row exists — decision 10).
2. Pick the target column from the action; stop if `null` or deleted.
3. Stop if a link of this task has `merge_moved_at` set (lock).
4. Lock the task row and stop unless the target column's position is greater than the current column's.
5. Move the task to the end of the target column through `tasks`' existing move logic, exposed as an exported method that takes the transaction (rule 6); `githubsync` depends on it through a small interface (rule 9).
6. Upsert the `task_pull_requests` link: `last_moved_event`, `moved_at`, and `merge_moved_at` when the event was a merge.

"Forward" means a greater column position, so the rule follows whatever order the board uses.

### 10. Data model (one migration, `00004_github_sync.sql`)

- `github_apps`: `id`, `token`, `github_app_id bigint`, `slug`, `owner_login`, `app_url`, `webhook_secret_ciphertext bytea`, `private_key_ciphertext bytea`, `connected_by` (user token), `created_at`, `updated_at`. At most one row: `CREATE UNIQUE INDEX ... ON github_apps ((true))`. **Disconnect deletes the row physically** — a documented exception to rule 22: a soft-deleted row would keep the private key in the database, which is exactly what disconnecting must prevent.
- `board_github_settings`: `id`, `token`, `board_id` (unique, FK), `opened_column_id`, `review_requested_column_id`, `merged_column_id` (nullable FKs to `board_columns`), `final_branches text[] NOT NULL`, `created_at`, `updated_at`. Created on the first `PATCH` from the defaults, then patched. It lives and dies with its board, so it has no `deleted_at`. Reads treat a soft-deleted column as `null`.
- `task_pull_requests`: `id`, `token`, `task_id` (FK), `repository` (`owner/name`), `pull_request_number int`, `last_moved_event text` (CHECK on the handled values), `moved_at`, `merge_moved_at` nullable, `created_at`, `updated_at`; `UNIQUE (task_id, repository, pull_request_number)`; index on `task_id`. These rows are how moves are attributed to Genwork rather than a user (spec "Movements attributed to Genwork").

Every table has `id` and `token` (rule 14); tokens use `platform.NewPublicToken`.

### 11. API surface and authorization

| Route | Policy |
|---|---|
| `GET /api/gentasks/v1/github/connection` | `gentasks.board.read` — returns no secret, and the board settings section needs it |
| `POST /api/gentasks/v1/github/connection/manifest` | `gentasks.github.manage` |
| `POST /api/gentasks/v1/github/connection` | `gentasks.github.manage` |
| `DELETE /api/gentasks/v1/github/connection` | `gentasks.github.manage` |
| `POST /api/gentasks/v1/github/webhook` | `httpapi.Public` + HMAC signature checked before parsing |
| `GET /api/gentasks/v1/boards/{token}/github-settings` | `gentasks.board.read` |
| `PATCH /api/gentasks/v1/boards/{token}/github-settings` | `gentasks.board.manage` |

`gentasks.github.manage` (default role `admin`) is declared in `module.yaml` and in `internal/access` (the existing test keeps both in step, rule 51). The webhook handler wraps the body in `http.MaxBytesReader` (5 MiB), reads the raw bytes, verifies the signature, and only then decodes JSON into request types (rules 11, 12). The `PATCH` request type distinguishes omitted from `null` with the SDK's existing patch helpers (rule 17).

### 12. Web

Inside `web/src/app/gentasks/` (rule 72):

- `datasource/githubDatasource.ts` — connection, manifest, completion, disconnect, board settings calls.
- `domain/githubSettings.ts` — building the minimal `PATCH` body, validating final branches; `domain/webhookReachability.ts` — the public URL warning rule; both unit-tested (rule 74).
- `components/GithubSettingsSection.tsx` (used by `BoardSettingsPage`), `components/useGithubConnection.ts`, and a helper that submits the manifest with a hidden form post (GitHub requires a `POST` with a `manifest` field).
- `GithubIntegrationPage.tsx` and `GithubCallbackPage.tsx`, routed at `/tasks/integrations/github` and `/tasks/integrations/github/callback`. TanStack Router ranks the static `integrations` segment above `$boardToken`, so they do not clash with board routes.
- Components come from `web/src/shared/ui/` (rule 73), colors from theme variables (rule 81), copy from i18n keys in pt-BR and English (rule 36).

## Risks / Trade-offs

- [GitHub cannot reach installations without a public HTTPS URL] → the integration page warns before connecting; polling is the planned follow-up and the private key is already stored for it.
- [Suggested App name already taken] → random suffix, and GitHub lets the admin edit the name before creating.
- [Admin creates the App on the wrong account and cannot install it on the org's repos] → the organization field explains why it matters; disconnecting and connecting again under the org fixes it.
- [`GENTASKS_ENCRYPTION_KEY` lost or replaced] → connection reads `unreadable`; the admin reconnects (and deletes the orphaned App on GitHub, which the page links to).
- [False positives such as `UTF-8` in a title] → only keys whose prefix is a real board and whose number is an active task move anything; the body is excluded.
- [Replay of a captured signed delivery] → idempotent processing (decision 7).
- [The whole change ships as one PR, exceeding rule 46's size cap] → exception approved by the developer; phases in tasks.md keep the review order and each phase ends green.

## Migration Plan

1. The installer change makes `genwork up` add `GENTASKS_ENCRYPTION_KEY`, `GITHUB_WEB_URL`, and `GITHUB_API_URL` to existing `.env` files before starting gentasks.
2. `00004_github_sync.sql` only adds tables, so it applies with no downtime.
3. A gentasks image started without the new variables fails at startup with an error naming them (rule 1).
4. Rollback: the migration's `Down` drops the three tables; the extra env variables are ignored by older images.
