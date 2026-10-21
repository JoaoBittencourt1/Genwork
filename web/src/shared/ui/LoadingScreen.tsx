import { useTranslation } from 'react-i18next'

export function LoadingScreen() {
  const { t } = useTranslation()
  return (
    <main className="flex min-h-screen items-center justify-center text-sm text-slate-500" aria-busy="true">
      {t('app.loading')}
    </main>
  )
}
