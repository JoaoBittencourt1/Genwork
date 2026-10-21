import type { UseQueryResult } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { formatTimeAgo } from '@/shared/domain/relativeTime'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Skeleton } from '@/shared/ui/skeleton'
import { taskIdentifier } from '../domain/taskPrefix'
import type { AssignedTask } from '../domain/types'
import { UrgencyIcon } from './UrgencyIcon'

export function HomeAssignedTasksSection({ assignedTasks }: { assignedTasks: UseQueryResult<AssignedTask[]> }) {
  const { t, i18n } = useTranslation()
  const now = new Date()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('home.assignedTasks.title')}</CardTitle>
        <CardDescription>{t('home.assignedTasks.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <ErrorAlert error={assignedTasks.error} />
        {assignedTasks.isPending ? <Skeleton className="h-24 w-full" /> : null}
        {assignedTasks.data?.length === 0 ? <p className="text-sm text-muted-foreground">{t('home.assignedTasks.empty')}</p> : null}
        {assignedTasks.data && assignedTasks.data.length > 0 ? (
          <ul className="-mx-2">
            {assignedTasks.data.map((task) => (
              <li key={task.token}>
                <Link
                  to="/tasks/$boardToken"
                  params={{ boardToken: task.board.token }}
                  className="flex items-start gap-2 rounded-md px-2 py-2 text-sm hover:bg-accent"
                >
                  <span className="mt-0.5" title={t(`gentasks.urgency.${task.urgency}`)}>
                    <UrgencyIcon urgency={task.urgency} />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex items-baseline gap-2">
                      <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{taskIdentifier(task.board.taskPrefix, task.number)}</span>
                      <span className="truncate font-medium">{task.title}</span>
                    </span>
                    <span className="block truncate text-xs text-muted-foreground">
                      {task.assignedAt
                        ? t('home.assignedTasks.assigned', { board: task.board.name, timeAgo: formatTimeAgo(task.assignedAt, now, i18n.language) })
                        : task.board.name}
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        ) : null}
      </CardContent>
    </Card>
  )
}
