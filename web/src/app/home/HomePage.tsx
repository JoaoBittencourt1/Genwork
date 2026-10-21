import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'

export function HomePage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  if (state.status !== 'signedIn') return null
  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('app.nav.home')}</h1>
      </PageHeader>
      <PageContent>
        <section className="space-y-2">
          <h2 className="text-2xl font-semibold tracking-tight">{t('home.greeting', { name: state.me.name })}</h2>
          <p className="text-muted-foreground">{t('home.description', { email: state.me.email, role: state.me.role.name })}</p>
        </section>
      </PageContent>
    </>
  )
}
