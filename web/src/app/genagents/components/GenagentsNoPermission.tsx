import { useTranslation } from 'react-i18next'

export function GenagentsNoPermission() {
  const { t } = useTranslation()
  return (
    <section className="space-y-2">
      <h2 className="text-2xl font-semibold tracking-tight">{t('genagents.noPermissionTitle')}</h2>
      <p className="text-muted-foreground">{t('genagents.noPermissionDescription')}</p>
    </section>
  )
}
