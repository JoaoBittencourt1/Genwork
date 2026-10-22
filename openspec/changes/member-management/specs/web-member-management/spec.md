## ADDED Requirements

### Requirement: Members page sections
The Members page SHALL show a members section to holders of `core.member.read` and the existing invites section (form and table) to holders of `core.invite.read`. The sidebar entry SHALL be visible to holders of either permission. A caller with neither SHALL see the existing "no permission" message.

#### Scenario: Admin opens Members
- **WHEN** an admin opens `/members`
- **THEN** the page shows the members table and, in its own section, the invite form and the invites table

#### Scenario: Invite-only access
- **WHEN** a caller holding `core.invite.read` but not `core.member.read` opens `/members`
- **THEN** only the invites section is shown

### Requirement: Members table
The members table SHALL show each member's avatar, name, e-mail, role, status, module access ("all modules" or the accessible ones), and last sign-in. The caller's own row SHALL be marked as theirs and offer no action. Deactivated members SHALL be visually distinguished with a status badge.

#### Scenario: Own row
- **WHEN** an admin views the table
- **THEN** their own row is marked "you" and has no role selector or action menu

### Requirement: Change a member's role
For holders of `core.member.manage`, each manageable row SHALL offer a role selector listing only the roles the caller's permissions cover. Choosing a role SHALL send `PATCH` with only `role`, and refresh the table on success.

#### Scenario: Demote an admin
- **WHEN** an admin selects `member` for another admin
- **THEN** the web app sends `{ "role": "<member token>" }` and the row shows `member` after the refresh

### Requirement: Module access dialog
For holders of `core.member.manage`, each manageable row SHALL offer a dialog that lists the modules enabled for the caller, each with a switch that is on when the member may access it. Saving SHALL send `PATCH` with only `restrictedModules`, built from the switches. Restricted modules the dialog does not list, such as disabled ones, SHALL be preserved. Module names SHALL come from translation keys, falling back to the module id.

#### Scenario: Remove access to a module
- **WHEN** an admin turns off `gentasks` for a member and saves
- **THEN** the web app sends `{ "restrictedModules": ["gentasks"] }` and the row's module access no longer lists it

#### Scenario: Disabled module preserved
- **WHEN** a member is restricted from a disabled module and an admin saves the dialog without touching it
- **THEN** the sent `restrictedModules` still contains the disabled module

### Requirement: Deactivate, reactivate, and remove
For holders of `core.member.manage`, each manageable row SHALL offer "deactivate" for active members, "reactivate" for deactivated members, and "remove" for all members. Deactivate and remove SHALL ask for confirmation naming the member, and remove SHALL explain that the member loses access and can only come back through a new invite. API errors SHALL be shown through the shared error alert with translated messages.

#### Scenario: Remove with confirmation
- **WHEN** an admin chooses "remove" on a member and confirms
- **THEN** the web app sends `DELETE` and the member disappears from the table

#### Scenario: Cancel
- **WHEN** an admin cancels the confirmation
- **THEN** no request is sent

### Requirement: Member rules live in the domain layer
Which actions a row offers, which roles are assignable, and how switches map to `restrictedModules` SHALL be pure functions in `web/src/app/members/domain/`, unit-tested (rule 74). Components SHALL only call them.

#### Scenario: Actions for a deactivated admin
- **WHEN** the domain computes actions for another, deactivated admin while the caller holds `core.member.manage`
- **THEN** it offers role change, module access, reactivate, and remove, but not deactivate
