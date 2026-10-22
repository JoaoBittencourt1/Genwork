## Context

Core already has users (with `deleted_at`), two seeded roles (`admin`, `member`), role permissions, sessions with revocation, and a module registry whose permissions carry a `module_id`. Access tokens embed the role's permissions, and modules authorize from them without calling core (rule 57). The web Members page only renders the invite form and the invites table.

Nothing in core today changes a user after they join. There is no status besides "deleted", no per-user permission adjustment, and no endpoint to act on a member.

## Goals / Non-Goals

**Goals:**
- Admins remove, deactivate/reactivate, change the role of, and restrict modules for other members.
- Module restrictions work for every module without core knowing any module by name (rule 52), and without module changes.
- Deactivation and removal cut core access at once.
- The dark theme stops tinting surfaces green.

**Non-Goals:** see proposal.md. In short: no custom roles, no module access at invite time, no instant revocation inside modules, deactivated users stay in the directory, no light-theme change.

## Decisions

### 1. A `members` feature that composes existing owners
New package `core/internal/members` (rule 4) with `MemberHandler`, `MemberService`, `UpdateMemberRequest`, and `MemberResponse`. It owns no table. It orchestrates the existing owners:
- `users.Repository`: gains `UpdateRole`, `UpdateDeactivatedAt`, and `SoftDelete`. `ListActive` is renamed `ListNotRemoved`, because "active" now names a status (rules 29, 30).
- `access`: roles, the new restriction repository, and `FindAssignableRole`, reused for the new role (rule 6).
- `sessions.Repository`: gains `RevokeAllForUser`.

The member list is three queries: users not removed, all roles, and the restrictions of those user ids in one batch. They are assembled in the service, which avoids N+1 (rule 21) and keeps one reader per table (rule 7).

Alternative considered: a single join query in a `MemberRepository`. It is faster by two round trips, but it duplicates how users and roles are read. That is not worth it for a page that loads tens of rows.

### 2. Status is derived from `users.deactivated_at`
Migration adds `deactivated_at timestamptz NULL`. `users.User` gains `DeactivatedAt` and `IsDeactivated()`. `members.MemberStatus` is a typed enum (`active`, `deactivated`, rule 15), derived from the column and never stored as a string, so the timestamp is the only source (rule 7). "Removed" keeps using `deleted_at` (rule 22).

### 3. Module access is a blocklist owned by `access`
Table `user_module_restrictions (id, token, user_id → users, module_id → modules, created_at, deleted_at)` with a unique index on `(user_id, module_id) WHERE deleted_at IS NULL`. Lifting a restriction soft-deletes its row (rule 22). Replacing the set is two statements in the member transaction: soft-delete rows not in the new set, then insert the missing ones with `ON CONFLICT DO NOTHING`.

- **Blocklist over allowlist:** with an allowlist, every newly installed module would need a grant per member before anyone could use it. That would make enabling a module depend on extra configuration, against rule 56, and it would change today's behavior. With a blocklist, "no row" means access, which is today's state, so the migration needs no backfill.
- **Owned by `access`:** module access is part of roles and permissions, which core owns (rule 49). Rows reference modules by internal `id` inside core (rule 14). The API speaks module ids (`modules.name`), the same ids clients get from capabilities.
- **Generic:** effective permissions use the existing `permissions.module_id` link. Core never names a module (rule 52).

### 4. Effective permissions in one place
`access.Service.FindUserPermissions(ctx, userID)` returns the role's non-retired permissions minus those whose `module_id` is restricted for the user. It is one query. It replaces `FindRolePermissions` behind the `auth` interface, renamed `UserPermissionFinder` (sessions already carry `UserID`), and feeds `/me`. Every token issuance path (login, invite acceptance, refresh) goes through `auth.Service.issueAccessToken`, so one call site covers all three.

Core's `RequirePermission` middleware keeps checking the role: core permissions have no `module_id`, so restrictions never change its answer, and it avoids an extra join on every request.

### 5. Capabilities filtered by composition
`CapabilitiesHandler` gains a second small interface, `RestrictedModuleFinder.FindRestrictedModuleNames(ctx, userID)` (implemented by `access.Service`). It removes those ids from `ListEnabled` (rule 9). `modules` does not read the restrictions table.

### 6. Deactivation, removal, and sign-in
- Deactivate and remove run in one transaction: the user update plus `RevokeAllForUser`.
- `sessions.FindActiveByToken` also requires `users.deactivated_at IS NULL`, so a concurrent request cannot slip through between the update and the revocation.
- Refresh is already refused because the sessions are revoked (`session_revoked`).
- Login runs the password verification as usual. A deactivated user then gets `invalid_credentials` with no failed-attempt count and no session. The work is the same as for a wrong password, so timing does not reveal the state (rule 68).
- Removal frees the e-mail through the existing partial unique index on `users(email) WHERE deleted_at IS NULL`. Invites sent by a removed member still show the inviter's name, because the invites query joins users without the `deleted_at` filter.

### 7. Guards in `MemberService`
One private method authorizes every mutation (rule 18): load the target, which returns `member_not_found` if it is missing or removed. Then reject the caller themselves (`cannot_manage_self`). Then require `callerRole.Covers(targetRole)` (`member_not_manageable`). A new role goes through `access.Service.FindAssignableRole` (`role_not_assignable`). Because the caller can never act on themselves and must already be an admin to manage anyone, at least one admin always remains. No separate "last admin" check is needed.

### 8. PATCH semantics in core
Core has no PATCH endpoint yet. `core/internal/platform` gains `PatchField[T]`, the same shape as `sdk/go/httpapi.PatchField`. Core does not import the SDK, which is the client side of core's contracts and depends on them, not the other way around. `UpdateMemberRequest` validates at the boundary (rule 11): present `null` → `400`, unknown `status` → `400`. It maps explicitly to a service input and never binds to `users.User` (rule 12). The required rule 17 test checks that a single-field body leaves the role, status, and restrictions unchanged.

### 9. Roles listing for both flows
`platform` gains a `RequireAnyPermission(...)` policy. The router and the `access` authorizer accept a list. `GET /api/core/v1/roles` moves to `RequireAnyPermission(core.invite.create, core.member.manage)`. Every other route keeps its single permission.

### 10. Web structure
- `web/src/app/members/`: `datasource/membersDatasource.ts` (list, patch, remove) and `domain/member.ts` (types, `memberActions`, `assignableRoles`, `toRestrictedModules`, `accessibleModules`). In `components/`: `MemberTable`, `MemberRoleSelect`, `MemberModuleAccessDialog`, `MemberActionsMenu`, and `queryKeys`. `MembersPage.tsx` renders the members section and the invites section.
- `listRoles` and the `Role` type move from the invites datasource to `web/src/shared/datasource/roles.ts`, with the `['roles']` query key next to them, because two features now use them (rule 4).
- Confirmations reuse `shared/components/ConfirmDialog`. Missing shadcn pieces (`switch`, maybe `tabs`) are added with `npx shadcn@latest add` (rules 73, 77).
- Module names come from `modules.names.<id>` in the locale files, falling back to the id.
- The module dialog lists the caller's capabilities. An admin restricted from a module therefore cannot change others' access to it, and `toRestrictedModules` keeps every restricted id the dialog does not show.
- The sidebar shows Members for `core.invite.read` or `core.member.read`.

### 11. Dark theme
In `.dark`, zero the chroma of every surface and text token listed in the `web-dark-theme` spec, keeping their lightness: background `oklch(0.13 0 0)`, card and popover `oklch(0.17 0 0)`, and so on. `--primary`, `--ring`, `--sidebar-primary`, and `--sidebar-ring` stay green. No component changes, because screens only use theme variables (rule 81).

## Risks / Trade-offs

- [A deactivated, removed, or newly restricted member keeps module access for up to 15 minutes, because modules validate tokens offline] → Accepted and documented in the spec. Core endpoints and refresh stop at once. Shortening the window would need a revocation feed for modules, which waits on the event bus decision.
- [The `RolePermissionFinder` → `UserPermissionFinder` rename touches the auth tests] → Mechanical. It is covered by the existing token tests plus new restriction tests.
- [`PatchField` exists in both core and the SDK] → Deliberate, because of the dependency direction. Both are tiny and tested where they live.
- [Renaming `ListActive` touches the directory service] → One call site, covered by the directory tests.
- [An admin restricted from a module cannot manage others' access to it from the UI] → Acceptable, since you cannot hand out access you do not have. The API still allows it for automation.

## Migration Plan

`00007_member_management.sql` is additive: one nullable column, one new table, and two permissions granted to `admin`. Existing users stay active with no restrictions, so behavior after deploy matches today's. The goose `Down` drops the table and the column and deletes the two permissions and their grants. Rollback loses only deactivations and restrictions made since the deploy.

## Open Questions

None blocking. The single-PR delivery in tasks.md needs the developer's approval, because it is an exception to rules 42, 44, and 46.
