import { InfoIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { type HostSnapshot, uptimeParts } from '../domain/hostSnapshot'

export function SystemCard({ snapshot }: { snapshot: HostSnapshot }) {
  const { t } = useTranslation()
  const unknown = t('genmonitor.system.unknown')
  const uptime = uptimeParts(snapshot.uptimeSeconds)
    .map((part) => t(`genmonitor.uptime.${part.unit}`, { count: part.count }))
    .join(' ')
  const rows = [
    { label: t('genmonitor.system.operatingSystem'), value: snapshot.operatingSystem.name || unknown },
    { label: t('genmonitor.system.kernel'), value: snapshot.operatingSystem.kernelRelease || unknown },
    { label: t('genmonitor.system.hostname'), value: snapshot.operatingSystem.hostname || unknown },
    { label: t('genmonitor.system.uptime'), value: uptime },
  ]
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <InfoIcon className="size-4" aria-hidden />
          {t('genmonitor.system.title')}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
          {rows.map((row) => (
            <div key={row.label} className="min-w-0">
              <dt className="text-sm text-muted-foreground">{row.label}</dt>
              <dd className="truncate font-medium" title={row.value}>
                {row.value}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}
