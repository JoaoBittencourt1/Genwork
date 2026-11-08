## Purpose

Frees disk space taken by earlier releases and removes transient rows nobody needs anymore.

## ADDED Requirements

### Requirement: Server images
A caller holding `core.system.update` SHALL see the images of earlier Genwork versions on the server, with their total size, and SHALL remove them. Only images of Genwork components from the configured registry whose tag is a version other than the installed one SHALL be removed. An image still in use SHALL be skipped and reported.

#### Scenario: After two updates
- **WHEN** the server runs 2.3.0 and still holds the 2.2.2 and 2.2.3 images
- **THEN** the cleanup lists and removes them and reports the bytes freed, and the 2.3.0 images stay

#### Scenario: Other images
- **WHEN** the server also holds `postgres:17-alpine` and `genwork-core:dev`
- **THEN** neither is listed nor removed

### Requirement: Runner cleanup
A caller holding `genagents.runner.manage` SHALL ask every runner to clean up. A runner SHALL remove its `genwork-sandbox` images of other versions and the workspaces of no active run, and report when it did and the bytes freed. A runner SHALL also remove sandbox images of other versions when it starts.

#### Scenario: Requested cleanup
- **WHEN** an admin frees space on the Runners page
- **THEN** each runner removes its old sandbox images on its next heartbeat and the Runners page shows the bytes freed

### Requirement: Transient rows
The same request SHALL delete sign-ins that ended over 7 days ago and OAuth authorizations that were used or expired over a day ago.

#### Scenario: Old sign-in
- **WHEN** a sign-in failed 8 days ago
- **THEN** the cleanup deletes it and the account keeps its status
