import { CpuIcon, HardDriveIcon, MemoryStickIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuth } from '@/app/auth/components/AuthContext'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { Spinner } from '@/shared/ui/spinner'
import { loadError } from '@/shared/domain/errors'
import { NetworkCard } from './components/NetworkCard'
import { SystemCard } from './components/SystemCard'
import { UsageCard } from './components/UsageCard'
import { useHostSnapshot } from './components/useHostSnapshot'
import { formatBytes, usedPercent } from './domain/hostSnapshot'
import { GENMONITOR_PERMISSIONS } from './domain/permissions'

export function HostPage() {
  const { t, i18n } = useTranslation()
  const { can } = useAuth()
  const canReadHost = can(GENMONITOR_PERMISSIONS.hostRead)
  const host = useHostSnapshot(canReadHost)
  const snapshot = host.data
  const bytes = (value: number) => formatBytes(value, i18n.language)

  return (
    <>
      <PageHeader>
        <h1 className="font-medium">{t('genmonitor.title')}</h1>
      </PageHeader>
      <PageContent className="flex flex-col gap-6">
        {!canReadHost ? (
          <section className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight">{t('genmonitor.noPermissionTitle')}</h2>
            <p className="text-muted-foreground">{t('genmonitor.noPermissionDescription')}</p>
          </section>
        ) : null}
        <ErrorAlert error={loadError(host)} />
        {canReadHost && host.isPending ? <Spinner /> : null}
        {snapshot ? (
          <>
            <p className="text-sm text-muted-foreground">
              {t('genmonitor.description', { seconds: snapshot.sampleIntervalSeconds })}
            </p>
            <div className="grid gap-4 md:grid-cols-3">
              <UsageCard title={t('genmonitor.cpu.title')} icon={CpuIcon} percent={snapshot.cpu.usedPercent}>
                {t('genmonitor.cpu.logicalCpus', { count: snapshot.cpu.logicalCpus })}
              </UsageCard>
              <UsageCard title={t('genmonitor.memory.title')} icon={MemoryStickIcon} percent={usedPercent(snapshot.memory)}>
                {t('genmonitor.usage', { used: bytes(snapshot.memory.usedBytes), total: bytes(snapshot.memory.totalBytes) })}
                <br />
                {t('genmonitor.available', { available: bytes(snapshot.memory.availableBytes) })}
              </UsageCard>
              <UsageCard title={t('genmonitor.disk.title')} icon={HardDriveIcon} percent={usedPercent(snapshot.disk)}>
                {t('genmonitor.usage', { used: bytes(snapshot.disk.usedBytes), total: bytes(snapshot.disk.totalBytes) })}
                <br />
                {t('genmonitor.available', { available: bytes(snapshot.disk.availableBytes) })}
              </UsageCard>
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <NetworkCard network={snapshot.network} />
              <SystemCard snapshot={snapshot} />
            </div>
          </>
        ) : null}
      </PageContent>
    </>
  )
}
