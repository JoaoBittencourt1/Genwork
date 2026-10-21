## Why

Genwork has no way to identify users yet, and every future module depends on it: core issues the tokens modules validate (constitution rule 57). Self-hosted installs also need a safe way to get the first administrator in and to bring other people in without opening public sign-up. This change delivers that foundation for the web app now, with contracts native clients can reuse later.

## What Changes

- **BREAKING (repo layout):** `server/` becomes `core/`, built as a single `core` binary with subcommands `serve`, `worker`, `migrate`, and `admin create`. Compose services are renamed to match.
- Core connects to PostgreSQL (own `core` schema) and runs goose migrations through `core migrate`.
- **Instance discovery:** `GET /.well-known/genwork` returns product id, instance name, version, API version, and available auth methods, so native clients can validate a server URL before showing a login screen.
- **Password authentication:** e-mail + password login issuing a short-lived signed access token (JWT) and a rotating refresh token bound to a session; refresh, logout, list and revoke own sessions; `GET /api/core/v1/me`. Passwords hashed with Argon2id; uniform errors, rate limiting, and account lockout (constitution rules 66–69).
- **Access control (minimal):** seeded `admin` and `member` roles and the first core permissions (`core.invite.create`, `core.invite.read`, `core.invite.revoke`), enforced on every endpoint that needs them.
- **Local stack commands:** `genwork up`, `db`, `api`, `web`, and `down` replace Makefile targets for running the stack.
- **Admin bootstrap:** new `cli/` component with `genwork auth`, which asks for e-mail, name, and password and creates an administrator through `core admin create` inside the running stack. There is no public sign-up and no first-access setup screen.
- **Invitations:** an authorized user invites an e-mail address with a role; Genwork sends the invite e-mail over SMTP and also returns the link for manual sharing. Invites are single-use, expire in 7 days, can be listed, revoked, and resent. Accepting an invite creates the account and signs the person in.
- **E-mail delivery:** SMTP mailer configured from the environment; Mailpit added to the local stack to capture e-mails during development.
- **Web app:** `web/` is scaffolded as a Vite + React SPA with login, invite acceptance, invite management, session restore, and pt-BR/en localization. In development it proxies `/api` and `/.well-known` to core so everything stays same-origin (constitution rule 65).
- CI, Makefile, and Dependabot updated for `core/`, `cli/`, and `web/`.

## Components touched

core, cli, web, deploy. No modules, no native clients.

## Capabilities

### New Capabilities

- `instance-discovery`: the public `/.well-known/genwork` document clients use to validate a server and learn how to sign in.
- `password-authentication`: login, token issuance, refresh rotation, logout, sessions, current-user endpoint, password hashing, rate limiting, and lockout.
- `access-control`: seeded roles, core permissions, and how endpoints enforce them.
- `admin-bootstrap`: creating administrators from the installer CLI.
- `invitations`: creating, listing, revoking, resending, e-mailing, and accepting invites.
- `web-auth-ui`: web screens and client behavior for signing in, accepting invites, managing invites, and keeping the session alive.

### Modified Capabilities

_(none — no specs exist yet)_

## Non-goals

- Password recovery, e-mail verification, and changing e-mail or password.
- Federated login (SSO/OIDC), MFA, passkeys.
- Workspaces, teams, and project membership; invites grant an instance-level role only.
- Role and permission management screens or endpoints beyond the seeded roles.
- The full installer (`genwork install`, module selection, image pulling) and the gateway; core is reached directly in development.
- Native client implementations; they will consume the contracts defined here in later changes.
- Signing key rotation.

## Impact

- **Code:** `server/` moved to `core/`; new `core/internal/{auth,users,sessions,invites,access,mail,discovery,platform}`; new `cli/`; new `web/`.
- **APIs:** `/.well-known/genwork`; `/api/core/v1/auth/*`, `/api/core/v1/me`, `/api/core/v1/roles`, `/api/core/v1/invites/*`, `/api/core/v1/invitations/*`.
- **Database:** new `core` schema with `users`, `roles`, `permissions`, `role_permissions`, `sessions`, `refresh_tokens`, and `invites` tables.
- **Dependencies:** pgx, goose, golang-jwt, golang.org/x/crypto (Argon2id), go-mail, golang.org/x/term; web: Vite, React, TanStack Router, Tailwind, i18next, Vitest, Testing Library.
- **Configuration:** new environment variables for database, public URL, instance name, JWT signing key, SMTP, and web dev proxy; documented in `.env.example` files.
- **Local stack:** Mailpit and a one-shot migration service added to `deploy/docker-compose.yml`.
- **Process:** delivered as a single phase at the developer's request — an explicit exception to constitution rules 42 and 46.
