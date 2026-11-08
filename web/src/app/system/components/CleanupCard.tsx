import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { HardDriveIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { formatBytes } from '@/shared/domain/byteUnits'
import { Button } from '@/shared/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Spinner } from '@/shared/ui/spinner'
import { findCleanupPreview, removeOutdatedImages } from '../datasource/updates'
import { canFreeSpace, cleanupOutcomeKey } from '../domain/serverCleanup'
import { systemQueryKeys } from './queryKeys'

// CleanupCard frees the disk space earlier Genwork versions left on the server: the updater removes their images.
export function CleanupCard() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const preview = useQuery({ queryKey: systemQueryKeys.cleanup, queryFn: findCleanupPreview })
  const [confirming, setConfirming] = useState(false)
  const removal = useMutation({ mutationFn: removeOutdatedImages, onSuccess: () => void queryClient.invalidateQueries({ queryKey: systemQueryKeys.cleanup }) })
  const bytes = (value: number) => formatBytes(value, i18n.language)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.cleanup.title')}</CardTitle>
        <CardDescription>
          {preview.isPending
            ? t('app.loading')
            : canFreeSpace(preview.data)
              ? t('system.cleanup.reclaimable', { size: bytes(preview.data?.totalBytes ?? 0), count: preview.data?.images.length ?? 0 })
              : t('system.cleanup.nothingToFree')}
        </CardDescription>
        {canFreeSpace(preview.data) ? (
          <CardAction>
            <Button variant="outline" size="sm" disabled={removal.isPending} onClick={() => setConfirming(true)}>
              {removal.isPending ? <Spinner /> : <HardDriveIcon />}
              {t('system.cleanup.action')}
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-2">
        <ErrorAlert error={preview.error ?? removal.error} />
        {removal.data ? <p className="text-sm text-muted-foreground">{t(cleanupOutcomeKey(removal.data), { size: bytes(removal.data.freedBytes), skipped: removal.data.skipped.length })}</p> : null}
      </CardContent>
      <ConfirmDialog
        open={confirming}
        title={t('system.cleanup.confirmTitle')}
        description={t('system.cleanup.confirmDescription')}
        confirmLabel={t('system.cleanup.action')}
        onOpenChange={setConfirming}
        onConfirm={() => {
          removal.mutate()
          setConfirming(false)
        }}
      />
    </Card>
  )
}
