## Purpose

Provides the installation's single public HTTP entrypoint, routing to core and to each module so that module containers are never exposed directly (rule 57).

## ADDED Requirements

### Requirement: Route by component prefix
The gateway SHALL forward `/api/core/...` and `/.well-known/...` to core, and `/api/<module>/v<N>/...` to the container of that module, keeping the full path. Module and core containers MUST NOT publish ports on the host, except the core port kept for local debugging.

#### Scenario: Core route
- **WHEN** a client calls `GET /api/core/v1/me` on the gateway
- **THEN** the request reaches core with the same path and headers

#### Scenario: Module route
- **WHEN** a client calls `GET /api/gentasks/v1/boards` on the gateway
- **THEN** the request reaches the gentasks container with the same path

#### Scenario: Unknown prefix
- **WHEN** a client calls `/api/unknown/v1/anything`
- **THEN** the gateway responds `404`

### Requirement: Internal paths blocked
The gateway SHALL respond `404` to any path starting with `/internal/` and never forward it.

#### Scenario: Registration route from outside
- **WHEN** a client calls `/internal/v1/modules/gentasks` on the gateway
- **THEN** the gateway responds `404`

### Requirement: Client address forwarded
The gateway SHALL set `X-Forwarded-For` to the address of the connecting client, and core SHALL trust it only from the gateway's network (rule 68).

#### Scenario: Spoofed header
- **WHEN** a client sends its own `X-Forwarded-For: 1.2.3.4` through the gateway
- **THEN** core rate-limits by the client's real address, not `1.2.3.4`
