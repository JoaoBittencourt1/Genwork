import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { isModuleEnabled, type ModuleId } from '@/shared/domain/capabilities'
import { Spinner } from '@/shared/ui/spinner'
import { useCapabilities } from './CapabilitiesContext'

export function RequireModule({ moduleId, children }: { moduleId: ModuleId; children: ReactNode }) {
  const { t } = useTranslation()
  const { enabledModules, isLoading } = useCapabilities()
  if (isLoading) return <Spinner />
  if (!isModuleEnabled(enabledModules, moduleId)) {
    return (
      <section className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t('modules.unavailableTitle')}</h1>
        <p className="text-muted-foreground">{t('modules.unavailableDescription')}</p>
      </section>
    )
  }
  return children
}
