# Genwork

A modular, self-hosted workspace platform designed to centralize task management, team workflows, infrastructure insights, and automation in a single ecosystem. Built with Go on the server and native desktop applications for macOS, Windows, and Linux, with optional modules, extensible integrations, and AI assistants connected through MCP.

## Repository layout

| Path | Description | Stack |
| --- | --- | --- |
| `core/` | Core service: auth, users, roles, invites, module registry | Go |
| `cli/` | `genwork` installer and operations CLI | Go |
| `mcp/` | `genwork-mcp`, the MCP server for AI assistants ([docs](mcp/README.md)) | Go |
| `web/` | Web app (static SPA) | Vite + React (TypeScript) |
| `clients/macos/` | macOS desktop app | Swift / SwiftUI |
| `clients/windows/` | Windows desktop app | C# / .NET (UI framework TBD) |
| `clients/linux/` | Linux desktop app | Rust / GTK |
| `deploy/` | Stack definition, gateway and updater images, `install.sh` | Docker Compose |
| `openspec/` | Specs and change proposals | OpenSpec |
| `.github/workflows/` | CI per component | GitHub Actions |

Every component ships in one platform release, whose version lives in [`VERSION`](VERSION). See [RELEASING.md](RELEASING.md).

## Installing on a server

A server needs Docker with the compose plugin, `curl`, and `tar`; no clone of the repository, Go, or Node. Run as the user that will own the installation:

```sh
curl -fsSL https://github.com/JoaoBittencourt1/Genwork/releases/latest/download/install.sh | GENWORK_HOME=/opt/genwork sh
```

`install.sh` downloads the `genwork` CLI and the deploy bundle of the release, checks both against the release's `checksums.txt` before writing anything, and runs `genwork install`. That asks for the public URL, the instance name, and optionally SMTP, generates every secret into `deploy/.env`, shows the module checklist, pulls the release's images, and starts the stack. Flags skip the questions: `genwork install --public-url https://genwork.example.com --instance-name Acme --modules gentasks`.

Then create the first administrator with `GENWORK_HOME=/opt/genwork genwork auth`. Set `GENWORK_HOME` in your shell profile to run `genwork` from any directory.

The gateway listens on `127.0.0.1:${GATEWAY_PORT}` (8487) and serves the web app, the APIs, and `/mcp`. Put your HTTPS reverse proxy in front of it, and make `GENWORK_PUBLIC_URL` the HTTPS address people open.

**Moving from a clone of the repository** (e.g. an existing staging server): run `genwork down` in the clone, then install with `--from-checkout`, which keeps the clone's `deploy/.env` (every secret) and module choice and reuses the same database volume:

```sh
curl -fsSL https://github.com/JoaoBittencourt1/Genwork/releases/latest/download/install.sh | GENWORK_HOME=/opt/genwork sh -s -- --from-checkout /srv/Genwork
```

Point the HTTPS proxy at the gateway port instead of the Vite port afterwards.

## Updating

Administrators see an update card on the web app's home page: "Check for updates", then "Update to vX.Y.Z". From the server:

```sh
genwork update --check   # reports a newer release, changes nothing
genwork update           # installs the latest release
```

Both run the same steps: download and verify the release, pull its images while the stack keeps running, back up the database into `backups/` (the five newest are kept), replace the deploy files, run migrations, restart, and wait until the gateway reports the new version. If anything after the backup fails, the update restores the previous files and database and starts the previous version again. If that restore fails too, `genwork update` prints the backup path and the commands to restore it by hand.

`genwork update` also replaces its own binary; when the binary's directory is not writable, it prints how to replace it. Updates from the web app run in the `updater` container, the only one with the Docker socket (rule 84). A clone of the repository updates through git instead.

## Connecting an AI assistant

Every member can open **AI assistants** in the web app's sidebar for the instance's MCP address and per-client steps. Claude (claude.ai and Desktop) adds it as a custom connector; Claude Code runs `claude mcp add --transport http genwork https://genwork.example.com/mcp`. The assistant signs in through the browser and acts with the user's own permissions. See [mcp/README.md](mcp/README.md).

## Running locally

Prerequisites: Docker with the compose and buildx plugins (the images build with BuildKit), Go 1.26, Node 26.

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
| `genwork up` | Starts database, gateway, API, installed modules, and mail catcher, then runs the web app in the foreground |
| `genwork db` | Starts PostgreSQL only |
| `genwork api` | Starts the gateway, core API, MCP server, updater, and installed modules (with database, migrations, worker, and Mailpit) |
| `genwork modules` | Opens the module checklist: checked modules are installed, unchecking one uninstalls it and deletes its data (asks first) |
| `genwork web` | Runs the web app dev server (installs dependencies on first run) |
| `genwork auth` | Creates an administrator; asks for e-mail, name, and password |
| `genwork down` | Stops the containers; data is kept |
| `genwork version` | Prints the version of the CLI |

The first `genwork up`, `db`, `api`, or `web` creates `deploy/.env` (with a new JWT signing key) and `web/.env` from their examples. The first `genwork up` or `api` also shows the module checklist; the choice is saved in `deploy/genwork.yaml` and can be changed any time with `genwork modules`. `deploy/genwork.yaml` and `deploy/Caddyfile` are generated for each installation and are not committed. In a clone, `genwork` adds `deploy/docker-compose.dev.yml`, which builds every image from source as `genwork-<name>:dev` and runs Mailpit; `deploy/docker-compose.yml` alone runs the published images of `VERSION`.

| Service | URL |
| --- | --- |
| Web app | http://localhost:8485 |
| Core API | http://localhost:8484 |
| Mailpit (captured e-mails) | http://localhost:8486 |
| Postgres | localhost:5484 |

Lint, tests, and builds run through `make lint`, `make test`, and `make build`. `make test` also enforces at least 75% coverage per component (core, sdk, modules, cli, mcp, web), and CI runs the same targets. Core tests need a database: `TEST_DATABASE_URL=postgres://genwork:change-me@localhost:5484/genwork?sslmode=disable make test` with `genwork db` running.

## Integrations

- [GitHub](modules/gentasks/docs/github-integration.md): pull requests move gentasks tasks between columns.

## Contributing

Read [constitution.md](constitution.md) first — its rules are non-negotiable. Features start as OpenSpec changes (`/opsx:propose`) under `openspec/`.
