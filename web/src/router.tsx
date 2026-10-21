import { createRootRoute, createRoute, createRouter, Outlet, type RouterHistory } from '@tanstack/react-router'
import { AppShell } from '@/app/auth/components/AppShell'
import { RequireAuth } from '@/app/auth/components/RequireAuth'
import { LoginPage } from '@/app/auth/LoginPage'
import { HomePage } from '@/app/home/HomePage'
import { AcceptInvitePage } from '@/app/invites/AcceptInvitePage'
import { MembersPage } from '@/app/members/MembersPage'

const rootRoute = createRootRoute({ component: Outlet })

const signedInLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'signed-in',
  component: () => (
    <RequireAuth>
      <AppShell>
        <Outlet />
      </AppShell>
    </RequireAuth>
  ),
})

const homeRoute = createRoute({ getParentRoute: () => signedInLayoutRoute, path: '/', component: HomePage })

const membersRoute = createRoute({ getParentRoute: () => signedInLayoutRoute, path: '/members', component: MembersPage })

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
  signedInLayoutRoute.addChildren([homeRoute, membersRoute]),
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
