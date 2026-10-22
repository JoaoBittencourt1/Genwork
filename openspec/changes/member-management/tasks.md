### Phased delivery

**Exception to rules 42, 44, and 46, pending the developer's approval:** the developer asked for the member management and the dark theme fix to ship together. So every phase below ships on the same branch, `feat/member-management`, in **one PR**. The phases still set the implementation and review order, each one starts with its tests, and each one ends with lint and tests green. The PR is opened only after the last phase and the developer's manual verification (rule 41). If the exception is refused, each phase gets its own branch (`feat/member-management-<phase slug>`) and PR, in the order of the table.

| Phase | Contents | Depends on | Parallel with |
|-------|----------|------------|---------------|
| 1 | core foundation: migration `00007`, `User.DeactivatedAt`, users/sessions/access repositories, `FindUserPermissions`, `platform.PatchField`, `RequireAnyPermission` | — | 4 |
| 2 | core enforcement: effective permissions in tokens and `/me`, filtered capabilities, login refuses deactivated members, roles route policy | 1 | 3, 4 |
| 3 | core members API: request type, `MemberService` with guards, handler, response type, routes | 1 | 2, 4 |
| 4 | web logic: shared roles datasource, members `datasource/` and `domain/` (API stubbed with MSW) | — (contracts in specs) | 1–3 |
| 5 | web UI: members table, role selector, module access dialog, actions menu, page sections, sidebar, i18n, neutral dark theme | 3, 4 | — |
| 6 | full gates, manual verification, single PR | 1–5 | — |

Go phases follow rule 47's order (test → migration → model → repository → authorization → request type → service → handler → response type). Core tests live under `core/tests/` (rule 71) with fixtures in dedicated files (rule 26).

## Phase 1 — core foundation

- [x] 1.1 Write tests: migration seeds `core.member.read` and `core.member.manage` for `admin` only; `users.Repository` `UpdateRole`, `UpdateDeactivatedAt` (set and clear), `SoftDelete`, and `ListNotRemoved` (includes deactivated, excludes removed); `sessions.Repository.RevokeAllForUser` revokes only that user's active sessions, and `FindActiveByToken` ignores sessions of deactivated users; `access` restriction repository replaces the set (soft-deletes lifted rows, inserts new ones, duplicates count once, unknown module name fails), lists restricted module names per user and for a batch of user ids; `FindUserPermissions` drops restricted modules' permissions, keeps core permissions, and ignores retired permissions; `platform.PatchField` tells omitted, `null`, and value apart; `RequireAnyPermission` allows any listed permission and refuses `403` otherwise
- [x] 1.2 Write migration `00007_member_management.sql`: `users.deactivated_at`, `user_module_restrictions` with its partial unique index, the two permissions and their `admin` grants, and a `Down` that reverses all of it
- [x] 1.3 Add `DeactivatedAt` and `IsDeactivated()` to `users.User` and the scan; rename `ListActive` to `ListNotRemoved` and update the directory service
- [x] 1.4 Implement the users, sessions, and access repository methods, and `access.Service.FindUserPermissions` and `FindRestrictedModuleNames`
- [x] 1.5 Add `PermissionMemberRead`/`PermissionMemberManage` to `access`, `platform.PatchField[T]`, and the `RequireAnyPermission` policy in the router and the access authorizer
- [x] 1.6 Run `make lint` and the core tests

## Phase 2 — core enforcement

- [x] 2.1 Write tests in `core/tests/app`: a member restricted from a registered test module gets tokens (login, refresh, invite acceptance) and `/me` without that module's permissions, and gets them back after the restriction is lifted; capabilities omit the restricted module for that member only; a deactivated member's correct-password login responds `401 invalid_credentials` with no session and no failed-attempt count; their refresh and access token are refused; `GET /roles` works with `core.member.manage` alone
- [x] 2.2 Rename `auth.RolePermissionFinder` to `UserPermissionFinder` and issue every access token with `FindUserPermissions`
- [x] 2.3 Return effective permissions from `/me`
- [x] 2.4 Make `auth.Service.Login` refuse deactivated users after password verification (rule 68)
- [x] 2.5 Give `CapabilitiesHandler` a `RestrictedModuleFinder` and filter the enabled modules by the caller; wire it in `internal/app/app.go`
- [x] 2.6 Move `GET /api/core/v1/roles` to `RequireAnyPermission(core.invite.create, core.member.manage)`
- [x] 2.7 Run `make lint` and the core tests

## Phase 3 — core members API

- [x] 3.1 Add member fixtures (admin caller, second admin, member, deactivated member, removed member, test module) in a dedicated helper file under `core/tests/app`
- [x] 3.2 Write tests: list shape and order, deactivated included, removed excluded, no internal fields, `403` without `core.member.read`; `PATCH` role, status, and `restrictedModules` each alone leave the other fields unchanged (rule 17); `null` and unknown values `400`; deactivation revokes every session and is idempotent; reactivation lets the member sign in again; `DELETE` responds `204`, revokes sessions, hides the member from the list and the directory, and lets the e-mail be invited again; guards `member_not_found`, `cannot_manage_self`, `member_not_manageable`, `role_not_assignable`, and `forbidden` with nothing changed
- [x] 3.3 Implement `UpdateMemberRequest` (boundary validation, explicit mapping, rules 11 and 12) and `MemberStatus`
- [x] 3.4 Implement `MemberService`: list (three batched queries, rule 21), update in one transaction, remove in one transaction, and the central guard (design §7)
- [x] 3.5 Implement `MemberHandler` and `MemberResponse`; register `GET /api/core/v1/members`, `PATCH` and `DELETE /api/core/v1/members/{token}` with their permissions (rule 18) in `internal/app/app.go`
- [x] 3.6 Run `make lint` and the core tests, and check core coverage stays ≥ 75% (rule 70)

## Phase 4 — web logic

- [x] 4.1 Add MSW handlers and fixtures for `GET/PATCH/DELETE /api/core/v1/members` in `web/src/test/`
- [x] 4.2 Write tests: datasource request shapes (`PATCH` body contains only the changed field) and error mapping (`cannot_manage_self`, `member_not_manageable`, `role_not_assignable`, `member_not_found`); `memberActions` for own row, active and deactivated members, a member the caller cannot manage, and a caller without `core.member.manage`; `assignableRoles` keeps only roles the caller's permissions cover; `toRestrictedModules` turns switches into ids and keeps restricted ids the dialog does not list; `accessibleModules` for the table summary
- [x] 4.3 Move `listRoles` and `Role` to `web/src/shared/datasource/roles.ts` and update the invites feature
- [x] 4.4 Implement `web/src/app/members/datasource/membersDatasource.ts` and `domain/member.ts`
- [x] 4.5 Run `make lint` and the web tests

## Phase 5 — web UI

- [x] 5.1 Add missing shadcn components (`switch`, and `tabs` if the layout needs it) with `npx shadcn@latest add` after checking `web/components.json` aliases (rule 73), and review the dependency diff (rule 77)
- [x] 5.2 Implement `MemberTable`, `MemberRoleSelect`, `MemberModuleAccessDialog`, `MemberActionsMenu` (with `ConfirmDialog` for deactivate and remove), and the members query keys
- [x] 5.3 Split `MembersPage` into the members section (`core.member.read`) and the invites section (`core.invite.read`); show the sidebar entry for either permission
- [x] 5.4 Add the pt-BR and English copy, including `modules.names.gentasks` and the new error codes
- [x] 5.5 Neutralize the dark theme surfaces in `web/src/index.css` (design §11)
- [ ] 5.6 Run `make lint` and the web tests

## Phase 6 — gates and PR

- [ ] 6.1 Run `make lint`, `make test`, and `make build` from the repo root (rule 75)
- [ ] 6.2 Read the whole diff against `main`, including generated files and lockfiles (rules 77, 80)
- [ ] 6.3 Stop for the developer's manual verification: change a member's role, restrict and restore `gentasks` access (sidebar and API after a refresh), deactivate and reactivate (sign-in refused, then allowed), remove and re-invite, and check the dark theme (rule 41)
- [ ] 6.4 Open the PR in pt-BR (rule 37) after sign-off
