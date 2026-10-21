import { CircleCheckIcon, CircleDashedIcon, CircleDotIcon, CircleIcon, type LucideIcon } from 'lucide-react'
import { cn } from '@/shared/lib/utils'
import { columnStatus, statusColorVariable, type ColumnStatus } from '../domain/columnName'
import type { Column } from '../domain/types'

const STATUS_ICONS: Record<ColumnStatus, LucideIcon> = {
  backlog: CircleDashedIcon,
  todo: CircleIcon,
  started: CircleDotIcon,
  done: CircleCheckIcon,
}

export function ColumnStatusIcon({ column, className }: { column: Pick<Column, 'preset'>; className?: string }) {
  const status = columnStatus(column)
  const Icon = STATUS_ICONS[status]
  return <Icon aria-hidden className={cn('size-3.5 shrink-0', className)} style={{ color: statusColorVariable(status) }} />
}
