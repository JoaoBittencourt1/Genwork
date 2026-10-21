import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'

export function HomePage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  if (state.status !== 'signedIn') return null
  return (
    <section className="space-y-2">
      <h1 className="text-2xl font-semibold tracking-tight">{t('home.greeting', { name: state.me.name })}</h1>
      <p className="text-muted-foreground">{t('home.description', { email: state.me.email, role: state.me.role.name })}</p>
    </section>
  )
}
