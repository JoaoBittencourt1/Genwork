import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/shared/lib/utils'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { Progress } from '@/shared/ui/progress'
import { formatPercent, usageLevel } from '../domain/hostSnapshot'

type UsageCardProps = {
  title: string
  icon: LucideIcon
  percent: number
  children: ReactNode
}

export function UsageCard({ title, icon: Icon, percent, children }: UsageCardProps) {
  const { i18n } = useTranslation()
  const isHigh = usageLevel(percent) === 'high'
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
          <Icon className="size-4" aria-hidden />
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <p className={cn('text-3xl font-semibold tabular-nums', isHigh && 'text-destructive')}>{formatPercent(percent, i18n.language)}</p>
        <Progress value={percent} aria-label={title} className={cn('h-2', isHigh && '[&_[data-slot=progress-indicator]]:bg-destructive')} />
        <div className="text-sm text-muted-foreground tabular-nums">{children}</div>
      </CardContent>
    </Card>
  )
}
