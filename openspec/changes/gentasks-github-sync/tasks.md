### Phased delivery

**Exception to rules 42, 44, and 46, approved by the developer:** every phase below ships on the same branch, `feat/gentasks-github-sync`, in **one PR**. The phases still set the implementation and review order, each one starts with its tests, and each one ends with lint and tests green. The PR is opened only after the last phase and the developer's manual verification (rule 41).

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | cli: per-module `<MODULE>_ENCRYPTION_KEY` generated and cleared by convention | — | 2, 6 |
| 2 | gentasks foundation: config and env example, migration `00004`, models, repositories, `gentasks.github.manage`, credential cipher | — | 1, 6 |
| 3 | gentasks connection API: manifest state, GitHub conversion client, connect/status/disconnect | 2 | 4, 6 |
| 4 | gentasks board GitHub settings API | 2 | 3, 6 |
| 5 | gentasks webhook: signature, key matching, movement rules | 2, 3, 4 | 6 |
| 6 | web logic: GitHub `datasource/` and `domain/` (API stubbed with MSW) | — (contracts in specs) | 1–5 |
| 7 | web UI: integration page, callback route, board settings section, i18n | 3, 4, 6 | — |
| 8 | full gates, manual verification, single PR | 1–7 | — |

Go phases follow rule 47's order (test → migration → model → repository → authorization → request type → service → handler → response type).

## Phase 1 — cli: per-module encryption key

- [x] 1.1 Write tests in `cli/internal/modules` and `cli/internal/commands`: `ConventionsFor("gentasks").EncryptionKeyVariable` is `GENTASKS_ENCRYPTION_KEY` (and hyphens map to underscores); `EnsureModuleSecrets` writes a 43-character base64url value when the variable is missing or empty, keeps an existing value, and leaves every other value untouched; `ClearModuleSecrets` empties it along with the database password and URL
- [x] 1.2 Add `EncryptionKeyVariable` to `ModuleConventions` and a 256-bit key generator next to `newURLSafeSecret`
- [x] 1.3 Fill the key in `EnsureModuleSecrets` and clear it in `ClearModuleSecrets`
- [x] 1.4 Run `make lint` and the CLI tests

## Phase 2 — gentasks foundation

- [x] 2.1 Write tests: config requires and validates `GENTASKS_ENCRYPTION_KEY` (decodes to ≥ 32 bytes), `GITHUB_WEB_URL`, and `GITHUB_API_URL`; the cipher round-trips, rejects a tampered ciphertext, rejects a ciphertext moved to another column or row (associated data), and fails with a different key; credential values print redacted through `fmt` and `slog`; repositories for `github_apps` (single row, physical delete), `board_github_settings` (create from defaults, patch, deleted column reads as null), and `task_pull_requests` (upsert per task and PR, lock lookup); the `module.yaml`/`access` permission test covers `gentasks.github.manage`
- [x] 2.2 Write migration `00004_github_sync.sql` with `github_apps`, `board_github_settings`, `task_pull_requests` (design §10), including the documented rule 22 exception comment on `github_apps`
- [x] 2.3 Add the config fields to `platform.ServeConfig`, and `GITHUB_WEB_URL=https://github.com`, `GITHUB_API_URL=https://api.github.com`, and an empty `GENTASKS_ENCRYPTION_KEY` to `deploy/.env.example`; point the test environment's GitHub URLs at an unroutable address (rule 25)
- [x] 2.4 Implement the models and repositories in `internal/githubsync/`
- [x] 2.5 Implement the AES-256-GCM credential cipher with HKDF key derivation (design §3) and the redacted credential types
- [x] 2.6 Declare `gentasks.github.manage` (default role `admin`) in `module.yaml` and `internal/access`
- [x] 2.7 Run `make lint` and the gentasks tests

## Phase 3 — gentasks connection API

- [x] 3.1 Add a fake GitHub server and recorded fixtures (manifest conversion success, `404`, `422`, `500`) in `internal/githubsynctest/` (rule 26)
- [x] 3.2 Write tests: manifest for a personal account and for an organization (form action, webhook/redirect/setup URLs, permissions, events, name length ≤ 34); invalid organization `400`; `409` when connected; state expired, tampered, or issued to another user `400` with no GitHub call; successful completion stores encrypted credentials and discards the client secret; GitHub `404`/`422` → `400`, failure or `5xx` → `502`; status `disconnected`/`connected`/`unreadable` with no secret in any response; disconnect deletes the row and returns `204` even when nothing is stored; `403` without `gentasks.github.manage`
- [x] 3.3 Implement the stateless manifest state signer (design §4)
- [x] 3.4 Implement the GitHub manifest conversion client behind a small interface, wired at the composition root (rule 9)
- [x] 3.5 Implement the request types, `GithubConnectionService`, `GithubConnectionHandler`, and response types; register the routes in `internal/app/app.go` with the policies of design §11
- [x] 3.6 Run `make lint` and the gentasks tests

## Phase 4 — gentasks board GitHub settings API

- [x] 4.1 Write tests: defaults for a never-configured board (merged = `done` preset, branches `main`, `master`) and for a board without a `done` column; deleted column reads as `null`; `404` for an unknown board; single-field `PATCH` leaves every other field unchanged, including unsaved defaults (rule 17); `null` clears a column; column from another board or deleted `400`; `finalBranches` empty, `null`, over 20 entries, or with whitespace `400`; duplicates removed; `403` without `gentasks.board.manage`
- [x] 4.2 Implement `UpdateBoardGithubSettingsRequest` with omitted/`null` handling through the SDK patch helpers
- [x] 4.3 Implement `BoardGithubSettingsService`, `BoardGithubSettingsHandler`, and the response type; register the routes
- [x] 4.4 Run `make lint` and the gentasks tests

## Phase 5 — gentasks webhook

- [x] 5.1 Add recorded webhook payload fixtures for `pull_request` `opened`, `edited`, `ready_for_review`, `review_requested`, `closed` merged into `main`, merged into `develop`, closed without merge, `labeled`, plus `ping` and `installation`, and a helper that signs a payload with a secret
- [x] 5.2 Write tests: missing, malformed, or wrong signature `401`; not connected or unreadable credentials `401`; body over 5 MiB `413`; ignored events and actions `204` with no move; key extraction from title and branch (case-insensitive, word boundaries, several keys, body ignored, unknown and archived/deleted tasks ignored) resolved in one query; each movement rule and scenario of the `gentasks-github-task-sync` spec (opened, review requested, merged into a final branch, other branch, closed without merge, never backwards, locked after merge, unconfigured event, redelivery idempotent); the link row records the moving event and `merge_moved_at`
- [x] 5.3 Export a transactional "move task to the end of a column" method from `tasks` that reuses the existing ordering logic, with its own tests in `tests/tasks`
- [x] 5.4 Implement the signature verifier and the task key matcher
- [x] 5.5 Implement `PullRequestSyncService` (design §9) and `GithubWebhookHandler` with `http.MaxBytesReader`; register the public route
- [x] 5.6 Run `make lint` and the gentasks tests, and check gentasks coverage stays ≥ 75% (rule 70)

## Phase 6 — web logic

- [x] 6.1 Add MSW handlers and fixtures for the GitHub endpoints in `web/src/test/`
- [x] 6.2 Write tests: datasource request shapes and error mapping for status, manifest, completion (`400` on `state`/`code`, `409`, `502`), disconnect, and board settings; `PATCH` body contains only changed fields and `null` for "do not move"; final branch validation (non-empty, ≤ 20, no whitespace, trimmed, deduplicated); webhook reachability warning for non-`https`, `localhost`, loopback, and private addresses, and none for a public `https` host
- [x] 6.3 Implement `datasource/githubDatasource.ts`, `domain/githubSettings.ts`, `domain/webhookReachability.ts`, and the types
- [x] 6.4 Run `make lint` and the web tests

## Phase 7 — web UI

- [x] 7.1 Add any missing shadcn components with `npx shadcn@latest add` after checking `web/components.json` aliases (rule 73) and review the dependency diff (rule 77)
- [x] 7.2 Implement `useGithubConnection`, the hidden-form manifest submit, `GithubIntegrationPage` (states, organization field, warning, install and settings links, disconnect confirmation), and `GithubCallbackPage`
- [x] 7.3 Add the routes `/tasks/integrations/github` and `/tasks/integrations/github/callback` and the boards-page entry for `gentasks.github.manage` holders
- [x] 7.4 Implement `GithubSettingsSection` in `BoardSettingsPage` (column selects with "do not move", final branches editor, read-only without `gentasks.board.manage`, not-connected notice with link for admins)
- [x] 7.5 Add the pt-BR and English copy
- [x] 7.6 Run `make lint` and the web tests

## Phase 8 — gates and PR

- [x] 8.1 Run `make lint`, `make test`, and `make build` from the repo root (rule 75)
- [x] 8.2 Read the whole diff against `main`, including generated files and lockfiles (rules 77, 80)
- [ ] 8.3 Stop for the developer's manual verification: connect an App through a public tunnel, install it on a test repository, and exercise opened, review requested, merged into a final branch, and the lock after merge (rule 41)
- [ ] 8.4 Open the single PR from `feat/gentasks-github-sync`, description in pt-BR (rule 37), stating the approved exceptions to rules 1, 22, 42, 44, 46, and 67
