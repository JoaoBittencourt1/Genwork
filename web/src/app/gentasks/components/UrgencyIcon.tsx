import { CircleAlertIcon, EllipsisIcon, SignalHighIcon, SignalLowIcon, SignalMediumIcon, type LucideIcon } from 'lucide-react'
import { cn } from '@/shared/lib/utils'
import { urgencyColorVariable } from '../domain/labelPalette'
import type { Urgency } from '../domain/types'

const URGENCY_ICONS: Record<Urgency, LucideIcon> = {
  none: EllipsisIcon,
  low: SignalLowIcon,
  medium: SignalMediumIcon,
  high: SignalHighIcon,
  urgent: CircleAlertIcon,
}

export function UrgencyIcon({ urgency, className }: { urgency: Urgency; className?: string }) {
  const Icon = URGENCY_ICONS[urgency]
  return <Icon aria-hidden className={cn('size-3.5 shrink-0', className)} style={{ color: urgencyColorVariable(urgency) }} />
}
