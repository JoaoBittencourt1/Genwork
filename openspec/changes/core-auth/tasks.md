### Phased delivery

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | Everything in this change: `server/` → `core/`, database and migrations, discovery, password authentication, access control, invites, e-mail, CLI admin bootstrap, web app, CI | — | — |

Single phase by explicit developer decision — an approved exception to constitution rules 42 and 46. Sections below follow dependency order (rule 47); each starts with its tests.

Branch: `feat(N-<issue>)/core-auth`

## 1. Repository restructure

- [x] 1.1 Move `server/` to `core/` and change the Go module path to `github.com/JoaoBittencourt1/Genwork/core`
- [x] 1.2 Replace `cmd/api` and `cmd/worker` with `cmd/core` dispatching `serve`, `worker`, `migrate`, `admin create`, and `keygen`; inject the version through `-ldflags`
- [x] 1.3 Remove the placeholder `internal/{auth,projects,tasks,modules}/doc.go` packages
- [x] 1.4 Update the Dockerfile to build the single `core` binary at `/core`
- [x] 1.5 Update the Makefile, `ci.yml` path filters, Dependabot, README, and CLAUDE.md for `core/`, `cli/`, and `web/`

## 2. Platform

- [x] 2.1 Write tests for config loading (missing required variables fail; SMTP username/password both-or-neither), client address resolution (direct peer, trusted proxy, spoofed header), the rate limiter, and the JSON error writer
- [x] 2.2 Implement `internal/platform`: config from environment, structured logger that never logs credentials, `pgxpool` setup, JSON error writer, client address resolver, in-memory rate limiter, public token generator
- [x] 2.3 Implement route registration with mandatory policies (`public`, `authenticated`, `permission(...)`) that fails at startup when a route has none

## 3. Database and migrations

- [x] 3.1 Write a test helper that migrates into a fresh random schema from `TEST_DATABASE_URL` and drops it afterwards; write a test that the migrations apply cleanly and seed `admin`/`member` with the three invite permissions
- [x] 3.2 Implement `core migrate` (create the `core` schema, set `search_path`, run embedded goose migrations)
- [x] 3.3 Write the migrations: `permissions`, `roles`, `role_permissions` (with seed data), `users`, `sessions`, `refresh_tokens`, `invites`, all with `id` + `token` and the indexes from design §3

## 4. Discovery

- [x] 4.1 Write handler tests for `GET /.well-known/genwork` (body and headers)
- [x] 4.2 Implement `internal/discovery` and register it as public

## 5. Users and passwords

- [x] 5.1 Write tests for Argon2id hashing (PHC format, parameter parsing, rehash detection, dummy verification) and password length validation in code points
- [x] 5.2 Write repository tests for creating and finding users by normalized e-mail, uniqueness under soft delete, and lockout fields
- [x] 5.3 Implement the password hasher with the concurrency semaphore
- [x] 5.4 Implement the users repository and service

## 6. Access control

- [x] 6.1 Write tests for loading role permissions, the permission middleware (`401` vs `403`), the role-subset rule, and `GET /api/core/v1/roles`
- [x] 6.2 Implement `internal/access`: roles repository, permission middleware, role-assignability check, roles handler

## 7. Sessions and tokens

- [x] 7.1 Write tests for JWT issue/verify (expired, wrong signature, wrong `iss`/`aud`), JWKS output, refresh rotation, reuse detection revoking the session, idle and absolute expiry, and revoked sessions rejected by the auth middleware
- [x] 7.2 Implement `core keygen` and Ed25519 key loading
- [x] 7.3 Implement `internal/sessions` (session and refresh-token repository and rotation in one transaction)
- [x] 7.4 Implement JWT issuing/verification, the JWKS handler, and the authentication middleware with the per-request session check

## 8. Login, refresh, logout, sessions, me

- [x] 8.1 Write HTTP tests for login (native body vs web cookie, uniform `invalid_credentials`, lockout after 10 failures, rehash on login, `last_login_at`), refresh (cookie + `X-Genwork-Client` for web, body for native), logout, list/revoke own sessions, `404` for others' sessions, `/me`, and `429` with `Retry-After`
- [x] 8.2 Implement the request/response types, `internal/auth` service, and handlers for login, refresh, logout, and sessions
- [x] 8.3 Implement `GET /api/core/v1/me`

## 9. E-mail

- [x] 9.1 Write tests rendering the invite templates in `pt-BR` and `en` (text and HTML contain instance name, inviter, link, expiry) and for SMTP config validation
- [x] 9.2 Implement `internal/mail` with `go-mail`, embedded localized templates, and a fake mailer for tests

## 10. Invites

- [x] 10.1 Write service and HTTP tests for create (`201` with link and `emailSent`, `email_already_registered`, `invite_already_pending`, `role_not_assignable`), delivery failure still creating the invite, derived status, list with status filter, revoke (`invite_not_pending`), resend invalidating the old link, inspect, accept (user created, signed in per client type), uniform `invite_not_found`, and concurrent acceptance
- [x] 10.2 Implement the invites repository (including `SELECT … FOR UPDATE` acceptance)
- [x] 10.3 Implement the invites service and handlers for `/api/core/v1/invites/*` and `/api/core/v1/invitations/*`

## 11. Admin bootstrap

- [x] 11.1 Write tests for `core admin create` (reads password from stdin, validates length, rejects existing e-mail, creates an active admin)
- [x] 11.2 Implement `core admin create --email --name --password-stdin`
- [x] 11.3 Create the `cli/` Go module with `cobra`; write tests for password prompt matching and the `docker compose exec` invocation (with a fake command runner)
- [x] 11.4 Implement `genwork auth` (asks for e-mail, name, password), including the "stack not running" error
- [x] 11.5 Implement `genwork db`, `api`, `web`, `up`, and `down`, with workspace discovery and `.env` creation; tests with a fake command runner

## 12. Local stack

- [x] 12.1 Add `mailpit`, `core-migrate`, `core`, and `core-worker` services to `deploy/docker-compose.yml`, all through `env_file`
- [x] 12.2 Update `deploy/.env.example` with every variable from design §10 and the Mailpit UI port `8486`
- [x] 12.3 Reduce the Makefile to lint, test, build, and `install-cli`; local setup lives in the CLI

## 13. Web app

- [x] 13.1 Scaffold `web/` with Vite, React, TypeScript, TanStack Router, TanStack Query, Tailwind, i18next, ESLint, Vitest, Testing Library, MSW, and `.nvmrc`; dev server proxy for `/api` and `/.well-known` from `DEV_API_PROXY_TARGET`, failing when missing; `web/.env.example`
- [x] 13.2 Write tests for the API client: bearer header, `X-Genwork-Client`, single shared refresh on concurrent `401`, retry, and logout on refresh failure
- [x] 13.3 Implement `shared/api` and the in-memory auth store; restore the session on load
- [x] 13.4 Write tests for the login screen (success to home, localized `invalid_credentials`, instance name from discovery)
- [x] 13.5 Implement the login and home screens with sign-out
- [x] 13.6 Write tests for invite acceptance (valid link, password mismatch blocked client-side, `invite_not_found` screen)
- [x] 13.7 Implement `/invite/:secret`
- [x] 13.8 Write tests for invite management (permission-gated navigation and controls, create showing link + copy + `emailSent`, resend, revoke, no-permission screen)
- [x] 13.9 Implement the invites screen
- [x] 13.10 Add `pt-BR` and `en` translations for every screen and every API error code in this change

## 14. CI and verification

- [x] 14.1 Extend `ci.yml` with `core` (Postgres service container, `TEST_DATABASE_URL`), `cli`, and `web` (lint, typecheck, test, build) jobs behind path filters; add `cli` gomod and `web` npm to Dependabot
- [x] 14.2 Run lint and tests for core, cli, and web
- [ ] 14.3 Manual end-to-end check: `genwork up`, `genwork auth`, sign in on the web, invite an e-mail, open it in Mailpit, accept in a private window, confirm the new member does not see Invites, revoke a session
- [ ] 14.4 Wait for developer sign-off (rule 41)
- [ ] 14.5 Open PR
