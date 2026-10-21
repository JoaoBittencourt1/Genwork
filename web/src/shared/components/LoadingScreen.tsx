import { useTranslation } from 'react-i18next'
import { Spinner } from '@/shared/ui/spinner'

export function LoadingScreen() {
  const { t } = useTranslation()
  return (
    <main className="flex min-h-svh items-center justify-center gap-2 text-sm text-muted-foreground" aria-busy="true">
      <Spinner />
      {t('app.loading')}
    </main>
  )
}
