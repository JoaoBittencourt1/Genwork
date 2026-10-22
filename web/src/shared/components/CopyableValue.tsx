import { CopyIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'

type CopyableValueProps = {
  value: string
  label: string
  copyLabel?: string
  copiedMessage?: string
  // multiline shows the value as a block, e.g. a JSON configuration.
  multiline?: boolean
}

export function CopyableValue({ value, label, copyLabel, copiedMessage, multiline = false }: CopyableValueProps) {
  const { t } = useTranslation()

  async function copyValue() {
    await navigator.clipboard.writeText(value)
    toast.success(copiedMessage ?? t('common.copied'))
  }

  const copyButton = (
    <Button variant="outline" onClick={() => void copyValue()} className="shrink-0">
      <CopyIcon />
      {copyLabel ?? t('common.copy')}
    </Button>
  )
  if (multiline) {
    return (
      <div className="flex flex-col gap-2">
        <pre aria-label={label} className="overflow-x-auto rounded-md border bg-muted px-3 py-2 font-mono text-xs">
          {value}
        </pre>
        <div>{copyButton}</div>
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-2 sm:flex-row">
      <Input readOnly aria-label={label} value={value} className="font-mono text-xs" />
      {copyButton}
    </div>
  )
}
