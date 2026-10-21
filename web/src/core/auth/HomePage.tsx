import { useTranslation } from 'react-i18next'
import { useAuth } from './AuthContext'

export function HomePage() {
  const { t } = useTranslation()
  const { state } = useAuth()
  if (state.status !== 'signedIn') return null
  return (
    <section>
      <h1 className="text-2xl font-semibold">{t('home.greeting', { name: state.me.name })}</h1>
      <p className="mt-2 text-slate-600">
        {t('home.description', { email: state.me.email, role: state.me.role.name })}
      </p>
    </section>
  )
}
