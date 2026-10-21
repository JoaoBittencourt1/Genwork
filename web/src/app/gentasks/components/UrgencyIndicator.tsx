import { SignalIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { urgencyColorVariable } from '../domain/labelPalette'
import type { Urgency } from '../domain/types'
import { urgencyTranslationKey } from '../domain/urgency'

export function UrgencyIndicator({ urgency }: { urgency: Urgency }) {
  const { t } = useTranslation()
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: urgencyColorVariable(urgency) }}>
      <SignalIcon className="size-3.5" aria-hidden />
      {t(urgencyTranslationKey(urgency))}
    </span>
  )
}
