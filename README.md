# Genwork

A modular, self-hosted workspace platform designed to centralize task management, team workflows, infrastructure insights, and automation in a single ecosystem. Built with Go on the server and native desktop applications for macOS, Windows, and Linux, with independently versioned releases, optional modules, and extensible integrations.

## Repository layout

| Path | Description | Stack |
| --- | --- | --- |
| `core/` | Core service: auth, users, roles, invites, module registry | Go |
| `cli/` | `genwork` installer and operations CLI | Go |
| `web/` | Web app (static SPA) | Vite + React (TypeScript) |
| `clients/macos/` | macOS desktop app | Swift / SwiftUI |
| `clients/windows/` | Windows desktop app | C# / .NET (UI framework TBD) |
| `clients/linux/` | Linux desktop app | Rust / GTK |
| `deploy/` | Local stack | Docker Compose |
| `openspec/` | Specs and change proposals | OpenSpec |
| `.github/workflows/` | CI per component | GitHub Actions |

Each component is versioned and released independently.

## Running locally

Prerequisites: Docker, Go 1.26, Node 26.

Install the `genwork` CLI once (it goes to `$(go env GOPATH)/bin`, which must be on your `PATH`):

```sh
make install-cli
```

If `genwork` is not found afterwards, add the Go bin directory to your `PATH` (zsh):

```sh
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

For bash, use `~/.bashrc` instead. If you changed `GOPATH`, replace `$HOME/go` with the output of `go env GOPATH`.

Then, from anywhere inside the repository:

| Command | What it does |
| --- | --- |
| `genwork up` | Starts database, API, and mail catcher, then runs the web app in the foreground |
| `genwork db` | Starts PostgreSQL only |
| `genwork api` | Starts the core API (with database, migrations, worker, and Mailpit) |
| `genwork web` | Runs the web app dev server (installs dependencies on first run) |
| `genwork auth` | Creates an administrator; asks for e-mail, name, and password |
| `genwork down` | Stops the containers; data is kept |

The first `genwork up`, `db`, `api`, or `web` creates `deploy/.env` (with a new JWT signing key) and `web/.env` from their examples.

| Service | URL |
| --- | --- |
| Web app | http://localhost:8485 |
| Core API | http://localhost:8484 |
| Mailpit (captured e-mails) | http://localhost:8486 |
| Postgres | localhost:5484 |

Lint, tests, and builds run through `make lint`, `make test`, and `make build`. `make test` also enforces at least 75% coverage per component (core, cli, web), and CI runs the same targets. Core tests need a database: `TEST_DATABASE_URL=postgres://genwork:change-me@localhost:5484/genwork?sslmode=disable make test` with `genwork db` running.

## Contributing

Read [constitution.md](constitution.md) first — its rules are non-negotiable. Features start as OpenSpec changes (`/opsx:propose`) under `openspec/`.
