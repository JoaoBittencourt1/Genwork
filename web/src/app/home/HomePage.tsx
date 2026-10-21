import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { PERMISSIONS } from '@/app/auth/domain/session'
import { HomeAssignedTasksSection } from '@/app/gentasks/components/HomeAssignedTasksSection'
import { HomeBoardsSection } from '@/app/gentasks/components/HomeBoardsSection'
import { useAssignedTasks } from '@/app/gentasks/components/useAssignedTasks'
import { useBoardList } from '@/app/gentasks/components/useBoardList'
import { useGentasksAbilities } from '@/app/gentasks/components/useGentasksAbilities'
import { useCapabilities } from '@/shared/components/CapabilitiesContext'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { isModuleEnabled, MODULE_IDS } from '@/shared/domain/capabilities'
import { useDirectoryUsers } from '@/shared/hooks/useDirectoryUsers'
import { CliCommandGuide } from './components/CliCommandGuide'
import { OnboardingChecklist } from './components/OnboardingChecklist'
import { buildOnboardingChecklist } from './domain/onboarding'

const HOME_ASSIGNED_TASKS_LIMIT = 3

export function HomePage() {
  const { t } = useTranslation()
  const { state, can } = useAuth()
  const { enabledModules } = useCapabilities()
  const gentasks = useGentasksAbilities()
  const showsGentasks = isModuleEnabled(enabledModules, MODULE_IDS.gentasks) && gentasks.canRead
  const boards = useBoardList(showsGentasks)
  const assignedTasks = useAssignedTasks(HOME_ASSIGNED_TASKS_LIMIT, showsGentasks)
  const directoryUsers = useDirectoryUsers(can(PERMISSIONS.inviteCreate) && can(PERMISSIONS.userRead))
  if (state.status !== 'signedIn') return null

  const checklist = buildOnboardingChecklist({
    gentasksEnabled: isModuleEnabled(enabledModules, MODULE_IDS.gentasks),
    can,
    boardCount: boards.data?.length,
    openAssignedTaskCount: assignedTasks.data?.length,
    directoryUserCount: directoryUsers.data?.length,
  })

  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('app.nav.home')}</h1>
      </PageHeader>
      <PageContent className="space-y-6">
        <section className="space-y-2">
          <h2 className="text-2xl font-semibold tracking-tight">{t('home.greeting', { name: state.me.name })}</h2>
          <p className="text-muted-foreground">{t('home.description', { email: state.me.email, role: state.me.role.name })}</p>
        </section>
        {checklist.totalCount > 0 ? <OnboardingChecklist checklist={checklist} /> : null}
        {showsGentasks ? (
          <div className="grid gap-6 md:grid-cols-2">
            <HomeBoardsSection boards={boards} canManageBoards={gentasks.canManageBoards} />
            <HomeAssignedTasksSection assignedTasks={assignedTasks} />
          </div>
        ) : null}
        <CliCommandGuide />
      </PageContent>
    </>
  )
}
