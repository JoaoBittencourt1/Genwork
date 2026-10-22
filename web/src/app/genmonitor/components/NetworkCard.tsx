import { ArrowDownIcon, ArrowUpIcon, NetworkIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { formatBytes, formatBytesPerSecond, type NetworkThroughput } from '../domain/hostSnapshot'

export function NetworkCard({ network }: { network: NetworkThroughput }) {
  const { t, i18n } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <NetworkIcon className="size-4" aria-hidden />
          {t('genmonitor.network.title')}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <dl className="grid grid-cols-2 gap-3">
          <div>
            <dt className="flex items-center gap-1 text-sm text-muted-foreground">
              <ArrowDownIcon className="size-4 text-primary" aria-hidden />
              {t('genmonitor.network.received')}
            </dt>
            <dd className="text-2xl font-semibold tabular-nums">{formatBytesPerSecond(network.receivedBytesPerSecond, i18n.language)}</dd>
          </div>
          <div>
            <dt className="flex items-center gap-1 text-sm text-muted-foreground">
              <ArrowUpIcon className="size-4" aria-hidden />
              {t('genmonitor.network.transmitted')}
            </dt>
            <dd className="text-2xl font-semibold tabular-nums">{formatBytesPerSecond(network.transmittedBytesPerSecond, i18n.language)}</dd>
          </div>
        </dl>
        <p className="text-sm text-muted-foreground tabular-nums">
          {network.interface
            ? t('genmonitor.network.totals', {
                name: network.interface,
                received: formatBytes(network.receivedBytes, i18n.language),
                transmitted: formatBytes(network.transmittedBytes, i18n.language),
              })
            : t('genmonitor.network.noDefaultRoute')}
        </p>
      </CardContent>
    </Card>
  )
}
