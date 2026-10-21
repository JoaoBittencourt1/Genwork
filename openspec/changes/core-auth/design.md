## Context

Toolchain: Go 1.26 (goose v3.29 requires it), Node 26.

The repository has a placeholder Go service in `server/` (an HTTP server with `/healthz` and an idle worker), a Postgres container, and no web app. The constitution already fixes the target layout (`core/`, `cli/`, `web/`), the identifier model (rule 14), the security baseline (rules 66–69), the one-entrypoint/token model for modules (rule 57), and that the web app is a static SPA on the same origin as the API (rule 65). The gateway and event bus are still open decisions and are not needed here.

Local ports already in use by this project: core `8484`, Postgres `5484`. This change adds web dev `8485` and Mailpit UI `8486`.

## Goals / Non-Goals

**Goals:**
- A single, reusable token model (access JWT + rotating refresh session) that the web app uses now and native clients can use later with no server changes.
- Invite-only onboarding with the first administrator created from the CLI.
- Real e-mail delivery through SMTP, exercised locally with Mailpit.
- A web app that proves the whole flow end to end.

**Non-Goals:** see the proposal. Notably: no password recovery, no SSO, no workspaces, no installer, no gateway.

## Decisions

### 1. One `core` binary with subcommands

`core/cmd/core/main.go` dispatches `serve`, `worker`, `migrate`, `admin create`, and `keygen`. One image runs as the API, the worker, and the one-shot migration job by changing the command.

- *Why:* the CLI needs to run privileged operations (creating an admin) using core's own password hashing — one implementation (rule 7). Running `core admin create` inside the container reuses it and the container's configuration, with no extra network surface.
- *Alternatives:* a bootstrap HTTP endpoint (rejected: it is a public path to create users, forbidden by the admin-bootstrap spec); the CLI writing to Postgres directly (rejected: duplicates hashing and couples the CLI to core's schema, against rule 53).

### 2. Package layout (rule 4)

```
core/
  cmd/core/                  subcommand dispatch, composition root
  internal/
    platform/                config, logging, HTTP server, DB pool, errors, client IP, rate limiter
    discovery/               /.well-known/genwork
    auth/                    login, refresh, logout, JWT issuing/verification, JWKS, middleware
    sessions/                session + refresh token persistence and rotation
    users/                   user persistence, password hashing, /me
    access/                  roles, permissions, permission middleware, /roles
    invites/                 invite lifecycle and acceptance
    mail/                    SMTP mailer, localized templates
  migrations/                goose SQL, embedded
```

Each feature package holds `handler.go`, `service.go`, `repository.go`, request/response types, and tests, named by role (rule 5). Packages talk through small interfaces declared by the consumer (rule 9); `cmd/core` wires them.

### 3. Database

- Driver: `pgx/v5` with `pgxpool`; repositories are handwritten SQL. *Alternative:* sqlc (generated, type-safe) — deferred to avoid another tool before there are enough queries to justify it.
- Migrations: goose, SQL files embedded with `embed`. `core migrate` creates the `core` schema if missing, sets `search_path` to it, and runs goose with its version table in that schema (rule 53). Migration SQL does not hard-code the schema name, so tests can migrate into throwaway schemas.
- Every table has `id bigint generated always as identity primary key` and `token text not null unique` (rule 14). Tokens are 16 random bytes, base64url without padding (22 chars).
- Tables (plural names avoid quoting the reserved word `user`):
  - `permissions(id, token, name unique)`
  - `roles(id, token, name unique)`
  - `role_permissions(id, token, role_id, permission_id, unique(role_id, permission_id))`
  - `users(id, token, email, name, password_hash, role_id, failed_login_count, locked_until, last_login_at, created_at, updated_at, deleted_at)` with a unique index on `email` `WHERE deleted_at IS NULL` (rule 22); e-mails are stored trimmed and lower-cased.
  - `sessions(id, token, user_id, client_type, device_name, created_at, last_used_at, idle_expires_at, absolute_expires_at, revoked_at)`
  - `refresh_tokens(id, token, session_id, token_hash unique, created_at, used_at)`
  - `invites(id, token, email, role_id, invited_by_user_id, secret_hash unique, created_at, expires_at, accepted_at, accepted_user_id, revoked_at)` with a plain index on `email`. "One pending invite per e-mail" depends on `expires_at`, which a partial index cannot express, so creation takes a per-e-mail transaction advisory lock and checks inside it.
- Public tokens come from a `public_token()` column default (pgcrypto `gen_random_bytes`), so the database is the single generator (rule 7).
- Seed data (roles, permissions) lives in migrations, since it is structural, not demo data.

### 4. Tokens and sessions (rule 69)

- **Access token:** JWT, EdDSA/Ed25519 via `golang-jwt/jwt/v5`, 15 minutes, claims `iss`, `aud`, `sub`, `sid`, `iat`, `exp`, header `kid`. The key is a base64 Ed25519 seed in `JWT_SIGNING_KEY`; `core keygen` prints a new one. Ed25519 gives small tokens and fast verification for future modules via `/api/core/v1/auth/jwks`.
- **Revocation:** core checks on every authenticated request that the `sid` session is active (one indexed lookup). Future modules will validate signatures only, so a revoked session can still reach a module for up to 15 minutes — accepted, and the reason access tokens are short.
- **Refresh:** 32 random bytes, base64url; only `sha256` stored (rule 67), compared in constant time. Each refresh marks the row `used_at` and inserts a new one in the same transaction. A presented token with `used_at` set revokes the session (theft detection).
- **Web transport:** refresh token in cookie `genwork_refresh` (`HttpOnly; Secure; SameSite=Strict; Path=/api/core/v1/auth`). Cookie-authenticated endpoints (`refresh`, `logout`) also require the header `X-Genwork-Client: web`, which a cross-site form cannot send.
- **Native transport:** refresh token in the JSON body; native clients will store it per server in the OS secure store, keyed by the server's public URL.

### 5. Passwords and abuse protection (rules 66, 68)

- `golang.org/x/crypto/argon2` with `m=64 MiB, t=3, p=2`, 16-byte salt, 32-byte key, PHC encoding. Verification parses parameters from the stored hash; a lower-than-current hash is rehashed after a successful login.
- A semaphore (capacity 4) bounds concurrent hashes so a burst of logins cannot exhaust memory on a small server (64 MiB each).
- Unknown e-mails are verified against a fixed dummy hash to equalize timing.
- Per-address rate limiting uses `golang.org/x/time/rate` token buckets in memory, keyed by the resolved client address; idle buckets are evicted. Client address is the TCP peer unless it is inside `TRUSTED_PROXY_CIDRS`, in which case it is the right-most untrusted `X-Forwarded-For` entry.
- Account lockout is persisted on `users` (`failed_login_count`, `locked_until`) so it survives restarts.

### 6. Authorization (rules 18, 60)

Routes are registered with an explicit policy: `public`, `authenticated`, or `permission("core.invite.create")`. Registration without a policy fails at startup. The permission middleware loads the caller's permission names (one join) per request. Role assignment checks the subset rule from the access-control spec.

### 7. Invites

- The link secret (32 random bytes, base64url) is distinct from the invite's public `token` (rule 14): the token identifies the invite in admin APIs; the secret is a bearer credential that only ever exists in the link, stored as `sha256` (rule 67).
- Status is derived from timestamps (rule 7).
- Acceptance runs in one transaction: `SELECT … FOR UPDATE` the pending invite by secret hash, create the user, set `accepted_at`, create the session.
- Creation commits first, then sends e-mail; a delivery failure is reported as `emailSent: false` and logged, and the admin can copy the link or resend.

### 8. E-mail

`wneessen/go-mail` (STARTTLS/TLS/auth support for real servers; plain for Mailpit). Templates are Go `text/template` and `html/template` files per locale under `core/internal/mail/templates/{pt-BR,en}/`, embedded. Locale comes from `GENWORK_DEFAULT_LOCALE`. Tests use a fake mailer (rule 25); Mailpit is only for running the stack.

### 9. API conventions

- Prefix `/api/core/v1` (rules 16, 57, 61); discovery and JWKS as specified.
- Errors: `{"error": {"code": "...", "fields": {...}}}` with codes only — clients translate (rule 36).
- JSON field names in camelCase.

### 10. Configuration (rules 1–3)

Core fails at startup when any required variable is missing:

| Variable | Purpose |
| --- | --- |
| `DATABASE_URL` | Postgres connection |
| `CORE_HTTP_ADDR` | listen address (replaces `API_ADDR`) |
| `GENWORK_PUBLIC_URL` | base for invite links and JWT `iss` |
| `GENWORK_INSTANCE_NAME` | shown in discovery and e-mails |
| `GENWORK_DEFAULT_LOCALE` | `pt-BR` or `en` |
| `JWT_SIGNING_KEY` | base64 Ed25519 seed |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_TLS` | mail delivery (`SMTP_TLS`: `none`, `starttls`, `tls`) |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | optional; both or neither |
| `TRUSTED_PROXY_CIDRS` | optional, comma-separated |

Web dev: `web/.env` with `DEV_API_PROXY_TARGET` and `DEV_SERVER_PORT`.

### 11. Local stack

`deploy/docker-compose.yml` services: `postgres`, `mailpit`, `core-migrate` (runs `migrate`, then exits), `core` (`serve`, depends on `core-migrate` completing successfully), `core-worker` (`worker`). All configuration through `env_file` (rule 2). The web app runs on the host with `npm run dev`; serving built static files belongs to the gateway decision.

### 12. CLI

`cli/` is its own Go module producing the `genwork` binary, built with `cobra` because the installer will grow many subcommands. It locates the repository by walking up to `deploy/docker-compose.yml` and is the single owner of local setup (creating `.env` files and the signing key), so the Makefile only lints, tests, and builds.

- `genwork db | api | up | down` wrap `docker compose`; `genwork web` and the tail of `genwork up` run `npm run dev` in the foreground, interrupting rather than killing it on Ctrl+C.
- `genwork auth` asks for e-mail and name (or takes `--email`/`--name`), reads the password twice with `golang.org/x/term`, then runs `docker compose exec -T core /core admin create --email … --name … --password-stdin`, writing the password to stdin.

### 13. Web app

- Vite + React + TypeScript, TanStack Router (typed routes), TanStack Query (server state), Tailwind CSS, i18next with `pt-BR` and `en`.
- Layout (rule 59): `web/src/core/{auth,invites,discovery}/` for core features, `web/src/shared/{api,ui,i18n}/` for the HTTP client, design system, and translations, `web/src/modules/` reserved for module UIs.
- `shared/api` owns the single fetch wrapper: adds the bearer token from the in-memory auth store, sends `X-Genwork-Client: web`, and on `401 invalid_token` runs one shared refresh promise, then retries.
- Tests: Vitest + Testing Library, with MSW stubbing the API (rule 25).
- Node version pinned in `web/.nvmrc`.

### 14. Testing strategy

- Go unit tests for pure logic: config, client address, rate limiter, router policies, password hashing, JWT, e-mail rendering, invite status.
- Services are exercised through HTTP integration tests (`internal/app`) against real Postgres from `TEST_DATABASE_URL`, with a controllable clock and a recording mailer; each test package migrates into a fresh random schema and drops it afterwards. Missing `TEST_DATABASE_URL` fails the run (rule 1). Faking repositories would mostly re-test the fakes, since the rules live in SQL conditions and transactions.
- CI (`ci.yml`) gains `core`, `cli`, and `web` jobs behind path filters; the `core` job runs a Postgres service container.

## Risks / Trade-offs

- [Single phase, well above ~600 lines and 10 files] → Explicit exception to rules 42 and 46 approved by the developer; tasks are still ordered by dependency so review can follow them in sequence.
- [In-memory rate limiting does not survive restarts or span multiple core replicas] → Acceptable for single-instance self-hosting; account lockout is persisted. Revisit if core is ever scaled horizontally.
- [`Secure` cookies over plain `http://localhost`] → Chromium and Firefox accept them on localhost; if a browser rejects them during development, run the dev server over HTTPS. Production is HTTPS.
- [Revoked sessions still valid at modules for up to 15 minutes] → Short access token lifetime; core itself checks the session on every request.
- [64 MiB per Argon2id hash on small servers] → Concurrency semaphore; parameters live in the hash, so they can be tuned later without breaking existing passwords.
- [SMTP failure blocks onboarding] → The link is always returned to the inviter for manual sharing.
- [Renaming `server/` to `core/` breaks local habits] → Done once, now, while there is no consumer.

## Migration Plan

No production deployments exist. Locally: `genwork down`, pull the change, delete the old `deploy/.env` so it is regenerated with the new variables, `genwork up`, then `genwork auth`. Rollback is `goose down` on a dev database or dropping the volume.

## Open Questions

- None blocking. The gateway choice will decide how built web assets are served in production; this change only runs the web app in dev mode.
