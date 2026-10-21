## Purpose

Lets modules announce themselves to core on startup so core can store their manifest facts, make their permissions assignable to roles, and publish the set of enabled modules to clients.

## ADDED Requirements

### Requirement: Module registration
Core SHALL accept a module registration at `PUT /internal/v1/modules/{id}` whose body carries the module's manifest facts: id, version, compatible core API range, API prefix, required modules, and the permissions it defines, each with the default roles it is granted to. The request MUST carry `Authorization: Bearer <GENWORK_MODULE_SECRET>`, compared in constant time. Registration SHALL be idempotent: registering the same id again updates the version, API prefix, and permission set, and records the registration time.

#### Scenario: First registration
- **WHEN** a module sends a valid manifest with the correct module secret
- **THEN** core responds `200`, stores the module as enabled, and returns the module's public token

#### Scenario: Re-registration with a new version
- **WHEN** a registered module registers again with a higher version
- **THEN** core stores the new version and the module stays enabled

#### Scenario: Wrong or missing module secret
- **WHEN** the registration request has no bearer secret or the wrong one
- **THEN** core responds `401` and changes nothing

#### Scenario: Path id and manifest id differ
- **WHEN** the `{id}` in the path differs from the manifest id
- **THEN** core responds `400`

#### Scenario: Incompatible core API range
- **WHEN** the manifest's core API range does not include core's API version
- **THEN** core responds `409` with code `incompatible_core_version` and does not enable the module

#### Scenario: Unsatisfied requirement
- **WHEN** the manifest requires a module that is not registered, or that is registered at a version outside the required range
- **THEN** core responds `409` with code `missing_requirement` and does not enable the module

### Requirement: Module permissions are namespaced and owned
Every permission in a manifest MUST be named `<module id>.<resource>.<action>`. Core SHALL store module permissions alongside core permissions, marked with their owning module, so roles can be granted them.

#### Scenario: Permission outside the module namespace
- **WHEN** the gentasks manifest declares `core.user.read` or `kanban.issue.create`
- **THEN** core responds `400` and stores nothing from that registration

#### Scenario: New permission granted to default roles once
- **WHEN** a module registers a permission core has not seen before, with default roles `admin` and `member`
- **THEN** core creates the permission and grants it to both roles

#### Scenario: Default grants are not re-applied
- **WHEN** an existing permission's grant to `member` was removed and the module registers again
- **THEN** core does not grant it to `member` again

#### Scenario: Permission removed from the manifest
- **WHEN** a module re-registers without a permission it declared before
- **THEN** core stops granting that permission to any role

#### Scenario: Unknown default role
- **WHEN** a manifest names a default role that does not exist
- **THEN** core responds `400`

### Requirement: Internal routes are not public
Core SHALL serve registration only under `/internal/`, and the gateway SHALL NOT forward any `/internal/` path.

#### Scenario: Internal route through the gateway
- **WHEN** a client calls `PUT /internal/v1/modules/gentasks` through the gateway
- **THEN** the gateway responds `404` without contacting core

### Requirement: Capabilities endpoint
Core SHALL expose `GET /api/core/v1/capabilities` to any authenticated user. It lists every enabled module with its id, version, and API prefix, plus core's own version and API version. Core MUST run and serve this endpoint with zero modules registered (rule 49).

#### Scenario: No modules
- **WHEN** an authenticated user calls the endpoint on an installation with no registered modules
- **THEN** core responds `200` with an empty module list and core's versions

#### Scenario: Gentasks registered
- **WHEN** gentasks has registered and a user calls the endpoint
- **THEN** the list contains `{ "id": "gentasks", "version": "0.1.0", "apiPrefix": "/api/gentasks/v1" }`

#### Scenario: Unauthenticated
- **WHEN** a request has no valid access token
- **THEN** core responds `401`
