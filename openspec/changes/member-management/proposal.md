## Why

The Members screen only manages invites: once someone accepts, an admin has no way to act on them. An admin cannot remove a person who left, temporarily block someone, promote or demote a member, or keep a member out of a module they should not use. The only workaround today is editing the database by hand, which rule 19 forbids.

The dark theme also tints every surface green (hue 150 on background, cards, sidebar, muted, and accent), so the app reads as "green-black" instead of black with a green accent.

## What Changes

- New core feature `internal/members` with an admin-only API to manage people who already joined:
  - `GET /api/core/v1/members` lists every member who has not been removed, with role, status (`active` or `deactivated`), joined and last sign-in dates, and the modules they cannot access.
  - `PATCH /api/core/v1/members/{token}` changes any of `role`, `status`, and `restrictedModules` (rule 17).
  - `DELETE /api/core/v1/members/{token}` removes a member (soft delete, rule 22); their e-mail can be invited again.
- Deactivating or removing a member revokes all of their sessions at once. A deactivated member cannot sign in or refresh and can be reactivated later with everything intact.
- **Per-member module access.** By default a member can use every enabled module their role allows. An admin can restrict a member from specific modules. Core then leaves that module's permissions out of the member's access token and `/me`, and leaves the module out of their capabilities. Modules need no change: they already authorize by token permissions (rule 57).
- Guards: nobody can manage themselves, and an admin can only manage a member whose current role, and the role being assigned, are covered by their own role (extends the existing no-escalation rule).
- New core permissions `core.member.read` and `core.member.manage`, granted to `admin`.
- Web: the Members page gains a members table with role selection, a module access dialog, deactivate/reactivate, and remove with confirmation. Invites move into their own section on the same page.
- Web: the dark theme's surfaces become neutral black and gray. Green stays only on `--primary`, `--ring`, and the sidebar primary (rule 81).

Components touched: **core** (migration, `access`, `auth`, `sessions`, `users`, `capabilities`, new `members` feature) and **web** (`web/src/app/members/`, `web/src/index.css`). No module, SDK, CLI, MCP, or native client changes. The change spans core and web because the screen is useless without the API (rule 63). The work is split into phases per component in tasks.md.

## Capabilities

### New Capabilities

- `member-management`: listing, changing role and status, and removing members; deactivated members cannot sign in; authorization and anti-escalation guards.
- `member-module-access`: per-member module restrictions, and how they shape effective permissions (access token, `/me`) and capabilities.
- `web-member-management`: the Members page UI for the above.
- `web-dark-theme`: neutral black surfaces in the dark theme, with green kept as the single accent.

### Modified Capabilities

None. No capability has been archived into `openspec/specs/` yet.

## Non-goals

- Custom roles or editing a role's permissions. Roles stay `admin` and `member`.
- Choosing module access while inviting. New members start with access to every module, and an admin restricts them afterwards.
- Instant revocation inside modules. Modules validate access tokens offline, so a deactivated, removed, or newly restricted member keeps module access until their current access token expires (at most 15 minutes, rule 69). Core endpoints and refresh stop working at once.
- Hiding deactivated members from the user directory. They stay in `GET /api/core/v1/users` so modules keep showing their names on existing records.
- Transferring or deleting a removed member's data inside modules.
- Changing the light theme.

## Impact

- `core/migrations/00007_member_management.sql`: `users.deactivated_at`, table `user_module_restrictions`, seeds `core.member.read` and `core.member.manage` for `admin`.
- `core/internal/access`: effective user permissions (role minus restricted modules), restriction repository.
- `core/internal/auth`: tokens are issued with effective permissions, and login refuses deactivated accounts.
- `core/internal/sessions`: revoke every session of a user, and active-session lookup ignores deactivated users.
- `core/internal/users`: `/me` returns effective permissions, and the repository gains role, status, and removal updates.
- `core/internal/capabilities`: the module list is filtered for the caller.
- `core/internal/members` (new): request types, service, handler, and response types.
- `web/src/app/members/`: datasource, domain, components, and page. `web/src/shared/i18n`: pt-BR and English copy. `web/src/index.css`: dark tokens.
- `mcp/`: no code change. It reads `/me` and capabilities, so restrictions apply to it automatically.
- No new Go dependencies. On the web, possibly the shadcn `switch` and `tabs` components (rule 73).
