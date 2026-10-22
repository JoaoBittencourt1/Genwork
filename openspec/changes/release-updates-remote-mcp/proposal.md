## Why

The first real installation (staging) showed that Genwork has no product to install. A server needs a clone of the repository: images are built on the server, the web app runs through the Vite dev server (a Node process in production, against rule 65), and the gateway does not serve the SPA. Nothing is published, so there is nothing to update to: every patch means pulling the repository and rebuilding by hand. Connecting an AI assistant has the same problem. `genwork-mcp` only runs over stdio, so each person needs the repository, Go, and a terminal command before Claude can use their Genwork.

## What Changes

- **Platform releases.**
  - **BREAKING (process):** every component ships under **one platform version**, kept in a root `VERSION` file. Rule 50 changes accordingly, and module manifests stop declaring their own `version`. Their registered version is the build's platform version.
  - Tagging `v<VERSION>` runs a release workflow. It runs the gates, then publishes multi-arch images to GHCR:
    - core
    - every module
    - the gateway: Caddy plus the built web bundle, so the gateway serves the SPA (rule 65)
    - the HTTP MCP server
    - the updater

    It also publishes the `genwork` and `genwork-mcp` binaries, a deploy bundle, `install.sh`, and `checksums.txt` as release assets.
  - The compose file splits in two. The base file references the published images, and a dev override adds `build:` and Mailpit. In a clone of the repository, `genwork up` keeps working as today.
  - `install.sh` and a new `genwork install` set up a server **without the repository**:
    1. download the CLI and the deploy bundle;
    2. generate `.env` with its secrets;
    3. ask for the public URL and the modules;
    4. pull the images and start the stack.
- **Updates.**
  - `genwork update` updates the installation:
    1. reads the latest GitHub release and compares it with the installed `VERSION`;
    2. verifies checksums and backs up Postgres;
    3. swaps the bundle, pulls the images, migrates, restarts, and waits for health;
    4. records the new version.

    If any step after the backup fails, it rolls back to the previous version and restores the backup. `genwork update --check` only reports whether a newer release exists. The CLI also replaces its own binary.
  - A new `updater` service runs the same update code. It is the only container that mounts the Docker socket. It has no route on the gateway and accepts requests only from core with a shared secret.
  - Core relays the updater to admins:
    - `GET /api/core/v1/system/update`: status;
    - `POST /api/core/v1/system/update/check`: look for a newer release;
    - `POST /api/core/v1/system/update`: install it.

    All three require the new permission `core.system.update`, granted to `admin`.
  - Web home page: for holders of `core.system.update`, a card shows the installed version and a "Check for updates" button. When a newer release exists, it shows "Update to vX.Y.Z" with the release notes link. While the update runs, it follows the progress and reloads the app when the new version answers.
- **Remote MCP.**
  - `genwork-mcp serve-http` serves MCP over Streamable HTTP at `https://<instance>/mcp` behind the gateway, reusing today's toolsets. Each request carries a core-issued access token. The server publishes OAuth protected resource metadata and answers `401` with a pointer to it when the token is missing or invalid.
  - Core becomes an OAuth 2.1 authorization server for assistants:
    - authorization server metadata;
    - dynamic client registration;
    - an authorization code flow with PKCE (S256);
    - a token endpoint that issues the usual 15-minute access tokens and rotating single-use refresh tokens (rule 69).

    Each grant is a session of a new `oauth` client type, named after the client (e.g. "Claude"), and is revocable like any session.
  - Web gains two pages:
    - a consent page (`/oauth/authorize`): "Claude wants to access Genwork as you";
    - a "Connect an AI assistant" page, reached from the sidebar and from a home card. It shows the instance's MCP URL with a copy button and steps for Claude (claude.ai and Desktop), Claude Code (`claude mcp add --transport http …`), and other MCP clients.

    The login page returns to the page that sent the user there.
- **Constitution amendments** in this change:
  - rule 49: core also owns the OAuth authorization server and the update relay;
  - rule 50: one platform version;
  - rule 51: `module.yaml` without `version`;
  - rule 57: the gateway also routes `/mcp` and serves the web bundle;
  - rule 64: the installer also installs and updates from releases;
  - new rule 84: only the updater touches the Docker socket;
  - the open decision on gateway technology is closed in favor of Caddy, which the stack already uses (pending the developer's approval).

Components touched: **core** (OAuth, update relay, migrations), **sdk** (manifest version from the build), **modules** gentasks and genmonitor (version injection only), **cli** (install, update, updater, compose split, gateway routes), **mcp** (HTTP transport), **web** (home update card, assistant pages, login return), **deploy** and **CI** (images, release workflow). The native clients are not touched. The change spans components because releases, updates, and remote MCP only work end to end. The developer asked for a single PR. Work is still split into phases per component in tasks.md, as an exception to rules 42, 44, and 46 that the developer has to approve.

## Capabilities

### New Capabilities

- `platform-releases`: single platform version, release artifacts and images, deploy bundle layout, installing a server from a release.
- `platform-updates`: checking for, applying, and rolling back updates from the CLI and the updater, and the admin update API in core.
- `web-platform-updates`: the update card on the home page.
- `oauth-authorization-server`: core as OAuth 2.1 authorization server for AI assistants (metadata, registration, consent, code and refresh grants, sessions).
- `mcp-remote-access`: MCP over Streamable HTTP at `/mcp`, protected resource metadata, per-user tools.
- `web-assistant-connection`: the consent page, the "Connect an AI assistant" page, and the login return path.

### Modified Capabilities

None. No capability has been archived into `openspec/specs/` yet.

## Non-goals

- TLS termination. The gateway keeps listening on HTTP behind the installation's own HTTPS proxy, as staging already does.
- Choosing a version other than the latest, downgrades, pre-release channels, and scheduled automatic updates. Updates are always started by an admin.
- Image signing (cosign) and SBOMs. Integrity relies on GHCR over HTTPS plus checksums of the downloaded assets.
- Migrating existing clones automatically. Moving staging from its clone to a release install is a documented manual step.
- An OAuth server for third-party apps in general: scopes, client secrets, and the client credentials grant. The only scope is acting as the signed-in user, for public clients with PKCE.
- Removing stdio `genwork-mcp`. It stays and is now downloaded from the release.
- Desktop clients, Windows packaging of the server, and Kubernetes.

## Impact

- New root `VERSION`. The Makefile and every Dockerfile inject it at build time. `module.yaml` files lose `version`, and `sdk/go/manifest` takes the version from the build.
- `deploy/`:
  - `docker-compose.yml` uses images;
  - new `docker-compose.dev.yml` keeps `build:` and Mailpit;
  - new services `mcp` and `updater`;
  - new `.env.example` keys: release channel, updater secret, `GENWORK_HOME`, MCP address;
  - new `deploy/gateway/Dockerfile`: Caddy plus the web bundle.
- `.github/workflows/release.yml` (new). New make targets build the release artifacts and images.
- `cli/`: `install`, `update`, and `updater` commands, a release client (GitHub API plus downloads with checksum verification), workspace kinds (repository and release installation, found through `GENWORK_HOME`), and Caddyfile routes for `/mcp` and the SPA.
- `core/`:
  - migrations for `core.system.update` and the OAuth tables (`oauth_clients`, `oauth_authorization_codes`, the `oauth` session client type);
  - new features `internal/updates` and `internal/oauth`;
  - new config `GENWORK_UPDATER_URL` and `GENWORK_UPDATER_SECRET`.
- `mcp/`: `serve-http` command, bearer verification with core's public keys (new dependency on `sdk/go`), and protected resource metadata. The version is injected at build time.
- `web/`:
  - `src/app/system/`: update card;
  - `src/app/assistants/`: consent and connect pages;
  - login return path;
  - Vite dev proxy for `/mcp`;
  - pt-BR and English copy.
- Docs: README (install and update), `mcp/README.md` (remote connection), new `RELEASING.md`.
- One-time manual steps for the developer: make the GHCR packages public after the first release, and move staging to a release installation.
