# Genwork

Project rules live in the constitution and are non-negotiable:

@constitution.md

## Commands

Run from the repo root:

- `make lint` / `make lint-fix` — core, cli, and web
- `make test` — core tests need `TEST_DATABASE_URL` (Postgres from `genwork db` on port 5484)
- `make build` — core binary, `genwork` CLI, web bundle
- `make install-cli` — install the `genwork` CLI into `$(go env GOPATH)/bin`

Running the stack (CLI, from anywhere in the repo): `genwork up | db | api | web | auth | down`.
