# Genwork

Project rules live in the constitution and are non-negotiable:

@constitution.md

## Commands

Run from the repo root:

- `make lint` / `make lint-fix` — core, sdk, modules, cli, mcp, and web
- `make test` — runs tests with coverage and fails below `MIN_COVERAGE` (75%, rule 70); core tests need `TEST_DATABASE_URL` (Postgres from `genwork db` on port 5484)
- `make build` — core and module binaries, `genwork` CLI, `genwork-mcp`, web bundle
- `make install-cli` / `make install-mcp` — install the `genwork` CLI / the `genwork-mcp` server into `$(go env GOPATH)/bin`

Running the stack (CLI, from anywhere in the repo): `genwork up | db | api | web | auth | modules | down`. `genwork modules` is the module checklist (checked = installed); the first `genwork up`/`api` shows it too.
