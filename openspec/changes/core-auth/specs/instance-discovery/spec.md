## ADDED Requirements

### Requirement: Discovery document
Core SHALL serve `GET /.well-known/genwork` without authentication, returning JSON with `product` (always `"genwork"`), `name` (the instance name from configuration), `version` (core version), `api` (current core API version, `"v1"`), and `auth.methods` (list of available sign-in methods, `["password"]` in this change).

#### Scenario: Client validates a Genwork server
- **WHEN** a client requests `GET /.well-known/genwork` on a running instance
- **THEN** core responds `200` with `Content-Type: application/json` and a body containing `product: "genwork"`, the configured instance name, the core version, `api: "v1"`, and `auth.methods: ["password"]`

#### Scenario: Discovery reveals nothing about users
- **WHEN** the discovery document is returned
- **THEN** it contains no user, session, or invite information

### Requirement: Discovery is cacheable and cross-origin readable
The discovery response SHALL include `Cache-Control: public, max-age=300` and `Access-Control-Allow-Origin: *`, so native clients and tools can read it from any origin.

#### Scenario: Headers on discovery response
- **WHEN** a client requests `GET /.well-known/genwork`
- **THEN** the response includes `Cache-Control: public, max-age=300` and `Access-Control-Allow-Origin: *`

### Requirement: Instance name is required configuration
Core SHALL refuse to start when `GENWORK_INSTANCE_NAME` is missing or blank.

#### Scenario: Missing instance name
- **WHEN** `core serve` starts without `GENWORK_INSTANCE_NAME`
- **THEN** the process exits with a non-zero status and logs that the variable is required
