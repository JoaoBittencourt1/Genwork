## Purpose

Turns the repository into installable releases: one platform version for every component, published images and binaries, and a way to install a server from a release without cloning the repository.

## ADDED Requirements

### Requirement: One platform version
Every component SHALL carry the same platform version, read from the root `VERSION` file at build time. Core SHALL report it as `version` in `GET /.well-known/genwork`. Every module SHALL register with core under that version, and the `genwork` and `genwork-mcp` binaries SHALL print it from their `version` command. `module.yaml` files SHALL NOT declare a version.

#### Scenario: Built components report the platform version
- **WHEN** the components are built from a checkout whose `VERSION` file contains `0.5.0`
- **THEN** `/.well-known/genwork` reports `0.5.0`, every module registers as `0.5.0`, and `genwork version` and `genwork-mcp version` print `0.5.0`

#### Scenario: A manifest that declares its own version is rejected
- **WHEN** a module's `module.yaml` contains a `version` key
- **THEN** reading the manifest fails with an error naming the unknown field

### Requirement: Releases are cut by tagging the platform version
Pushing a tag `v<version>` SHALL publish a release only if `<version>` equals the content of `VERSION` at the tagged commit and lint, tests, and builds pass. Otherwise nothing SHALL be published.

#### Scenario: Tag matches VERSION
- **WHEN** the tag `v0.5.0` is pushed on a commit whose `VERSION` is `0.5.0` and the gates pass
- **THEN** a GitHub release `v0.5.0` is published with every release asset and image

#### Scenario: Tag does not match VERSION
- **WHEN** the tag `v0.5.1` is pushed on a commit whose `VERSION` is `0.5.0`
- **THEN** the workflow fails before building, and no image or release is published

### Requirement: Release contents
A release SHALL publish:
- images for `linux/amd64` and `linux/arm64`, tagged with the version: core, every module, the gateway (with the built web app), the HTTP MCP server, and the updater;
- `genwork` binaries for Linux and macOS (amd64 and arm64);
- `genwork-mcp` binaries for Linux, macOS, and Windows (amd64 and arm64);
- the deploy bundle;
- `install.sh`;
- `checksums.txt` with the SHA-256 of every other asset.

#### Scenario: Every asset is covered by the checksums
- **WHEN** a release is published
- **THEN** `checksums.txt` lists one SHA-256 for each binary archive, the deploy bundle, and `install.sh`

### Requirement: Deploy bundle layout
The deploy bundle SHALL contain only what a server needs to run the release, laid out like the repository: `VERSION`, `deploy/docker-compose.yml`, `deploy/.env.example`, and `modules/<id>/module.yaml` for every module. It SHALL NOT contain source code, the development compose override, or web sources.

#### Scenario: Bundle content
- **WHEN** the deploy bundle of `v0.5.0` is extracted
- **THEN** it contains `VERSION` with `0.5.0`, the base compose file referencing images tagged `0.5.0`, the env example, and one manifest per module, and nothing else

### Requirement: The gateway serves the web app
In a release installation, the gateway SHALL serve the built web app on every path that no API, `/.well-known`, or `/mcp` route claims, answering with `index.html` for paths that match no file so client-side routes load. `index.html` SHALL be served with `Cache-Control: no-cache`, and hashed assets SHALL be served as immutable. `/internal/*` SHALL keep answering `404`.

#### Scenario: Deep link to a client route
- **WHEN** a browser requests `/tasks/abc` from the gateway of a release installation
- **THEN** the gateway answers `200` with the web app's `index.html`

#### Scenario: API routes keep precedence
- **WHEN** a client requests `/api/core/v1/capabilities` or `/.well-known/genwork`
- **THEN** the gateway proxies the request to core, not to the web app

#### Scenario: Internal routes stay closed
- **WHEN** a client requests `/internal/v1/modules/gentasks` through the gateway
- **THEN** the gateway answers `404`

### Requirement: Installing a server from a release
`install.sh` SHALL install the `genwork` binary for the machine's OS and architecture, extract the deploy bundle of the chosen release (the latest by default) into the install directory (`GENWORK_HOME`), verify both against `checksums.txt`, and run `genwork install`. `genwork install` SHALL:
- create `deploy/.env` from the example with fresh secrets;
- ask for the public URL (an absolute `http` or `https` URL) and the instance name, or take them from flags;
- let the operator choose modules and record the choice;
- pull the images of the installed version and start the stack;
- enable the chosen modules.

It SHALL NOT build images or run Node.

#### Scenario: Fresh server
- **WHEN** an operator runs `install.sh` on a Linux server with Docker and answers the prompts
- **THEN** the stack runs the release's images, the gateway serves the web app at the public URL, and `genwork auth` can create the first administrator

#### Scenario: Checksum mismatch
- **WHEN** a downloaded asset does not match its entry in `checksums.txt`
- **THEN** installation stops before writing anything into the install directory and reports which asset failed

#### Scenario: Existing installation
- **WHEN** `genwork install` runs in a directory that already has `deploy/.env` and a recorded module choice
- **THEN** it refuses, keeps every file unchanged, and points to `genwork update`

#### Scenario: Moving from a repository checkout
- **WHEN** an operator runs `genwork install --from-checkout /srv/Genwork` on a server whose stack ran from that clone
- **THEN** the installer copies the checkout's `deploy/.env` (keeping every secret) and module choice, adds keys the release needs, starts the release's images on the same database volume, and keeps the existing data

#### Scenario: Invalid public URL
- **WHEN** the operator enters `genwork.example.com` as the public URL
- **THEN** the installer asks again and explains that the URL must start with `http://` or `https://`

### Requirement: The CLI tells repository checkouts and release installations apart
The CLI SHALL find its installation through `GENWORK_HOME` when set, and otherwise through the current directory or its parents. In a repository checkout, `genwork up`, `api`, and `web` SHALL keep building images locally, running the web dev server, and starting Mailpit. In a release installation, `genwork up` and `api` SHALL start the published images only, and `genwork web` SHALL refuse with an explanation.

#### Scenario: Running from anywhere on a server
- **WHEN** `GENWORK_HOME=/opt/genwork` and the operator runs `genwork update` from their home directory
- **THEN** the CLI acts on the installation in `/opt/genwork`

#### Scenario: Development checkout keeps working
- **WHEN** a developer runs `genwork up` inside a clone of the repository
- **THEN** images are built from source, Mailpit starts, and the web dev server runs in the foreground, as before this change
