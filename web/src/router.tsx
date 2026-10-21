import { createRootRoute, createRoute, createRouter, Outlet, type RouterHistory } from '@tanstack/react-router'
import { AppShell } from '@/app/auth/components/AppShell'
import { RequireAuth } from '@/app/auth/components/RequireAuth'
import { LoginPage } from '@/app/auth/LoginPage'
import { BoardPage } from '@/app/gentasks/BoardPage'
import { BoardsPage } from '@/app/gentasks/BoardsPage'
import { HomePage } from '@/app/home/HomePage'
import { AcceptInvitePage } from '@/app/invites/AcceptInvitePage'
import { MembersPage } from '@/app/members/MembersPage'
import { CapabilitiesProvider } from '@/shared/components/CapabilitiesProvider'
import { RequireModule } from '@/shared/components/RequireModule'
import { MODULE_IDS } from '@/shared/domain/capabilities'

const rootRoute = createRootRoute({ component: Outlet })

const signedInLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'signed-in',
  component: () => (
    <RequireAuth>
      <CapabilitiesProvider>
        <AppShell>
          <Outlet />
        </AppShell>
      </CapabilitiesProvider>
    </RequireAuth>
  ),
})

const homeRoute = createRoute({ getParentRoute: () => signedInLayoutRoute, path: '/', component: HomePage })

const membersRoute = createRoute({ getParentRoute: () => signedInLayoutRoute, path: '/members', component: MembersPage })

const boardsRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/tasks',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.gentasks}>
      <BoardsPage />
    </RequireModule>
  ),
})

const boardRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/tasks/$boardToken',
  component: function BoardRoute() {
    const { boardToken } = boardRoute.useParams()
    return (
      <RequireModule moduleId={MODULE_IDS.gentasks}>
        <BoardPage key={boardToken} boardToken={boardToken} />
      </RequireModule>
    )
  },
})

const loginRoute = createRoute({ getParentRoute: () => rootRoute, path: '/login', component: LoginPage })

const acceptInviteRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/invite/$secret',
  component: function AcceptInviteRoute() {
    const { secret } = acceptInviteRoute.useParams()
    return <AcceptInvitePage secret={secret} />
  },
})

const routeTree = rootRoute.addChildren([
  signedInLayoutRoute.addChildren([homeRoute, membersRoute, boardsRoute, boardRoute]),
  loginRoute,
  acceptInviteRoute,
])

export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
