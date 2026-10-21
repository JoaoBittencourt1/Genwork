import { createRootRoute, createRoute, createRouter, Outlet, type RouterHistory } from '@tanstack/react-router'
import { AppShell } from '../core/auth/AppShell'
import { HomePage } from '../core/auth/HomePage'
import { LoginPage } from '../core/auth/LoginPage'
import { RequireAuth } from '../core/auth/RequireAuth'
import { AcceptInvitePage } from '../core/invites/AcceptInvitePage'
import { InvitesPage } from '../core/invites/InvitesPage'

const rootRoute = createRootRoute({ component: Outlet })

const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => (
    <RequireAuth>
      <AppShell>
        <HomePage />
      </AppShell>
    </RequireAuth>
  ),
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

const invitesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/invites',
  component: () => (
    <RequireAuth>
      <AppShell>
        <InvitesPage />
      </AppShell>
    </RequireAuth>
  ),
})

const routeTree = rootRoute.addChildren([homeRoute, loginRoute, acceptInviteRoute, invitesRoute])

export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
