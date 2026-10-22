## Purpose

Keeps a release installation current: operators and admins can check for a newer platform release and install it, from the CLI or the web app, with a database backup and an automatic rollback when the update fails.

## ADDED Requirements

### Requirement: Checking for a newer release
Checking SHALL compare the installation's `VERSION` with the latest published release of the configured release channel, ignoring drafts and pre-releases. It SHALL report either that the installation is up to date or the newer version with its release notes URL and publication date. A failed check SHALL change nothing and SHALL report the failure.

#### Scenario: Newer release available
- **WHEN** the installation runs `0.5.0` and the latest release is `v0.5.2`
- **THEN** `genwork update --check` reports `0.5.2` with its release notes URL and exits with status 0 without changing anything

#### Scenario: Already up to date
- **WHEN** the installation runs `0.5.2` and the latest release is `v0.5.2`
- **THEN** the check reports that the installation is up to date

#### Scenario: Release channel unreachable
- **WHEN** the GitHub API cannot be reached during a check
- **THEN** the check reports `release_channel_unreachable` and the recorded latest release stays as it was

### Requirement: Applying an update
`genwork update` and the updater SHALL install the latest release only when it is newer than the installed version. Before changing anything, they SHALL download the release assets, verify every checksum, and pull the new images, so a failure there leaves the installation untouched. Then they SHALL go through these steps, in order:
1. back up the database;
2. keep a copy of the current deploy files;
3. replace the deploy files with the new bundle, keeping `deploy/.env`, `deploy/genwork.yaml`, and the backups;
4. add new `.env` keys with generated secrets, and regenerate the gateway routes;
5. run the migrations and restart the services;
6. wait until every service runs and the gateway reports the new version.

They SHALL write the new `VERSION` only after the last step succeeds.

#### Scenario: Successful update
- **WHEN** an update from `0.5.0` to `0.5.2` runs and every step succeeds
- **THEN** the installation's `VERSION` is `0.5.2`, `/.well-known/genwork` reports `0.5.2`, a backup of the database taken before the update exists in `backups/`, and the status is `succeeded`

#### Scenario: Nothing newer to install
- **WHEN** an update is requested while the installation already runs the latest release
- **THEN** it is refused with `no_newer_release` and nothing changes

#### Scenario: Corrupted download
- **WHEN** the downloaded deploy bundle does not match `checksums.txt`
- **THEN** the update stops with `checksum_mismatch` before the backup, and the installation keeps running the old version untouched

#### Scenario: Image pull fails
- **WHEN** pulling one of the new images fails
- **THEN** the update stops with step `pull` and `rolledBack: false` because nothing was changed, and the installation keeps running the old version

#### Scenario: Secrets for new configuration
- **WHEN** the new release's `.env.example` adds a secret variable that the installation's `.env` lacks
- **THEN** the update adds it with a generated value and keeps every existing value unchanged

### Requirement: Automatic rollback
If any step after the backup fails, the update SHALL restore the previous deploy files, restore the database from the backup taken by this update, and start the previous version again. The status SHALL then be `failed` with the failing step and `rolledBack: true`. If the rollback itself fails, the status SHALL be `failed` with `rolledBack: false`, and the CLI SHALL print the path of the backup and the manual recovery steps.

#### Scenario: Migration fails
- **WHEN** a new migration fails during an update from `0.5.0` to `0.5.2`
- **THEN** the database is restored from the pre-update backup, the stack runs `0.5.0` again, `VERSION` stays `0.5.0`, and the status reports step `migrate` with `rolledBack: true`

#### Scenario: Rollback fails
- **WHEN** restoring the backup fails after a failed update
- **THEN** the status reports `rolledBack: false`, and `genwork update` prints the backup path and the restore commands

### Requirement: One update at a time
The CLI and the updater SHALL share one lock per installation. An update or check requested while an update runs SHALL be refused with `update_in_progress`.

#### Scenario: Concurrent request
- **WHEN** an admin starts an update from the web app while `genwork update` is running on the server
- **THEN** the second request is refused with `update_in_progress` and the running update continues

### Requirement: Backup retention
Each update SHALL write a database backup to `backups/` in the installation, named with the previous version and a timestamp. It SHALL keep the five most recent backups and delete older ones only after a successful update.

#### Scenario: Old backups are pruned
- **WHEN** a successful update leaves six backups in `backups/`
- **THEN** the oldest one is deleted and the five newest remain

### Requirement: Repository checkouts are not updated
In a repository checkout, `genwork update` SHALL refuse with an explanation that the checkout updates through git. The updater SHALL report `updatable: false` with reason `development_installation` and refuse to apply updates. Checking for a newer release SHALL still work.

#### Scenario: Update in a clone
- **WHEN** a developer runs `genwork update` inside a clone of the repository
- **THEN** the CLI refuses without touching containers, files, or the database

### Requirement: The CLI updates itself
After a successful update, `genwork update` SHALL replace its own binary with the release's binary for its OS and architecture, verified against `checksums.txt`. When the binary is not writable, it SHALL leave the binary unchanged and print how to replace it. A `genwork` binary older than the installation's `VERSION` SHALL warn on every command that it is out of date.

#### Scenario: Binary in a protected directory
- **WHEN** the update succeeds but the user cannot write to `/usr/local/bin/genwork`
- **THEN** the installation is updated, and the CLI prints that its own binary is still the old version and how to replace it

### Requirement: The updater service
The updater SHALL run as a container of the stack. It SHALL be the only container that mounts the Docker socket. It SHALL listen only on the stack's internal network, and the gateway SHALL have no route to it. Every request SHALL carry the updater secret, compared in constant time, or be refused with `401`. The secret SHALL never appear in logs or responses. The status SHALL survive the updater's own restart, and the updater SHALL replace its own container last, after the update has been recorded.

#### Scenario: Request without the secret
- **WHEN** a container on the internal network calls the updater without the updater secret
- **THEN** the updater answers `401` and does nothing

#### Scenario: Status after the updater replaced itself
- **WHEN** an update to `0.5.2` finishes and the updater restarts with its `0.5.2` image
- **THEN** its status reports `succeeded` with installed version `0.5.2`

### Requirement: Admin update API in core
Core SHALL relay update status, checks, and update requests to the updater through these endpoints:
- `GET /api/core/v1/system/update`: current status;
- `POST /api/core/v1/system/update/check`: check for a newer release;
- `POST /api/core/v1/system/update` with body `{"version": "<x.y.z>"}`: apply.

Each endpoint SHALL require `core.system.update`, which core SHALL grant to `admin` only. The status response SHALL contain:
- `installedVersion`;
- `updatable`, plus a reason when it is false;
- `latest`, either `{version, notesUrl, publishedAt}` or `null`;
- `checkedAt`;
- `state`: `idle`, `updating`, `succeeded`, or `failed`;
- during and after an update, `targetVersion`, `step`, `startedAt`, and `finishedAt`;
- for a failure, `failure: {code, step, rolledBack}`.

An apply request SHALL be refused with `409 version_mismatch` unless `version` equals the latest known release.

#### Scenario: Admin checks for updates
- **WHEN** an admin calls `POST /api/core/v1/system/update/check`
- **THEN** core answers `200` with the status after the check, including `latest`

#### Scenario: Member without permission
- **WHEN** a member without `core.system.update` calls any of the three endpoints
- **THEN** core answers `403 forbidden` and the updater receives no request

#### Scenario: Stale version in the request
- **WHEN** an admin requests `{"version": "0.5.1"}` while the latest known release is `0.5.2`
- **THEN** core answers `409 version_mismatch` and no update starts

#### Scenario: Update accepted
- **WHEN** an admin requests the latest known version and no update runs
- **THEN** core answers `202` with `state: "updating"` and the target version

#### Scenario: Updater unreachable
- **WHEN** the updater does not answer
- **THEN** core answers `503 updater_unavailable`
