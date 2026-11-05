import { createRootRoute, createRoute, createRouter, Outlet, type RouterHistory } from '@tanstack/react-router'
import { AuthorizeAssistantPage } from '@/app/assistants/AuthorizeAssistantPage'
import { ConnectAssistantPage } from '@/app/assistants/ConnectAssistantPage'
import { AppShell } from '@/app/auth/components/AppShell'
import { RequireAuth } from '@/app/auth/components/RequireAuth'
import { LoginPage } from '@/app/auth/LoginPage'
import { BoardPage } from '@/app/gentasks/BoardPage'
import { BoardSettingsPage } from '@/app/gentasks/BoardSettingsPage'
import { BoardsPage } from '@/app/gentasks/BoardsPage'
import { GithubCallbackPage } from '@/app/gentasks/GithubCallbackPage'
import { GithubIntegrationPage } from '@/app/gentasks/GithubIntegrationPage'
import { AccountsPage } from '@/app/genagents/AccountsPage'
import { AdminChatPage } from '@/app/genagents/AdminChatPage'
import { BotsPage } from '@/app/genagents/BotsPage'
import { McpServersPage } from '@/app/genagents/McpServersPage'
import { RunnersPage } from '@/app/genagents/RunnersPage'
import { SkillsPage } from '@/app/genagents/SkillsPage'
import { HostPage } from '@/app/genmonitor/HostPage'
import { HomePage } from '@/app/home/HomePage'
import { AcceptInvitePage } from '@/app/invites/AcceptInvitePage'
import { ForgotPasswordPage } from '@/app/passwordreset/ForgotPasswordPage'
import { ResetPasswordPage } from '@/app/passwordreset/ResetPasswordPage'
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

const assistantsRoute = createRoute({ getParentRoute: () => signedInLayoutRoute, path: '/assistants', component: ConnectAssistantPage })

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

const boardSettingsRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/tasks/$boardToken/settings',
  component: function BoardSettingsRoute() {
    const { boardToken } = boardSettingsRoute.useParams()
    return (
      <RequireModule moduleId={MODULE_IDS.gentasks}>
        <BoardSettingsPage key={boardToken} boardToken={boardToken} />
      </RequireModule>
    )
  },
})

const githubIntegrationRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/tasks/integrations/github',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.gentasks}>
      <GithubIntegrationPage />
    </RequireModule>
  ),
})

const githubCallbackRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/tasks/integrations/github/callback',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.gentasks}>
      <GithubCallbackPage />
    </RequireModule>
  ),
})

const hostRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/host',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genmonitor}>
      <HostPage />
    </RequireModule>
  ),
})

const aiAccountsRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/ai/accounts',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genagents}>
      <AccountsPage />
    </RequireModule>
  ),
})

const aiBotsRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/ai/bots',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genagents}>
      <BotsPage />
    </RequireModule>
  ),
})

const aiSkillsRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/ai/skills',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genagents}>
      <SkillsPage />
    </RequireModule>
  ),
})

const aiMcpServersRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/ai/mcp-servers',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genagents}>
      <McpServersPage />
    </RequireModule>
  ),
})

const aiRunnersRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/ai/runners',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genagents}>
      <RunnersPage />
    </RequireModule>
  ),
})

const aiChatRoute = createRoute({
  getParentRoute: () => signedInLayoutRoute,
  path: '/ai/chat',
  component: () => (
    <RequireModule moduleId={MODULE_IDS.genagents}>
      <AdminChatPage />
    </RequireModule>
  ),
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>): { redirect?: string } =>
    typeof search.redirect === 'string' ? { redirect: search.redirect } : {},
  component: LoginPage,
})

// The consent screen keeps the focused sign-in layout, outside the app shell, as the login does.
const authorizeAssistantRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/oauth/authorize',
  component: () => (
    <RequireAuth>
      <AuthorizeAssistantPage />
    </RequireAuth>
  ),
})

const acceptInviteRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/invite/$secret',
  component: function AcceptInviteRoute() {
    const { secret } = acceptInviteRoute.useParams()
    return <AcceptInvitePage secret={secret} />
  },
})

const forgotPasswordRoute = createRoute({ getParentRoute: () => rootRoute, path: '/forgot-password', component: ForgotPasswordPage })

const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reset-password/$secret',
  component: function ResetPasswordRoute() {
    const { secret } = resetPasswordRoute.useParams()
    return <ResetPasswordPage secret={secret} />
  },
})

const routeTree = rootRoute.addChildren([
  signedInLayoutRoute.addChildren([
    homeRoute,
    membersRoute,
    assistantsRoute,
    boardsRoute,
    boardRoute,
    boardSettingsRoute,
    githubIntegrationRoute,
    githubCallbackRoute,
    hostRoute,
    aiAccountsRoute,
    aiBotsRoute,
    aiSkillsRoute,
    aiMcpServersRoute,
    aiRunnersRoute,
    aiChatRoute,
  ]),
  loginRoute,
  authorizeAssistantRoute,
  acceptInviteRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
])

export function createAppRouter(history?: RouterHistory) {
  return createRouter({ routeTree, history })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
