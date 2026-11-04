import type { UseQueryResult } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { KanbanSquareIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Skeleton } from '@/shared/ui/skeleton'
import { loadError } from '@/shared/domain/errors'
import type { BoardSummary } from '../domain/types'

type HomeBoardsSectionProps = {
  boards: UseQueryResult<BoardSummary[]>
  canManageBoards: boolean
}

export function HomeBoardsSection({ boards, canManageBoards }: HomeBoardsSectionProps) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('home.boards.title')}</CardTitle>
        <CardDescription>{t('home.boards.description')}</CardDescription>
        {boards.data && boards.data.length > 0 ? (
          <CardAction>
            <Button variant="ghost" size="sm" asChild>
              <Link to="/tasks">{t('home.boards.viewAll')}</Link>
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-2">
        <ErrorAlert error={loadError(boards)} />
        {boards.isPending ? <Skeleton className="h-24 w-full" /> : null}
        {boards.data?.length === 0 ? (
          <div className="flex flex-col items-start gap-3">
            <p className="text-sm text-muted-foreground">{t('home.boards.empty')}</p>
            {canManageBoards ? (
              <Button size="sm" variant="outline" asChild>
                <Link to="/tasks">{t('home.boards.create')}</Link>
              </Button>
            ) : null}
          </div>
        ) : null}
        {boards.data && boards.data.length > 0 ? (
          <ul className="-mx-2">
            {boards.data.map((board) => (
              <li key={board.token}>
                <Link
                  to="/tasks/$boardToken"
                  params={{ boardToken: board.token }}
                  className="flex items-center gap-2 rounded-md px-2 py-2 text-sm hover:bg-accent"
                >
                  <KanbanSquareIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                  <span className="min-w-0 flex-1 truncate font-medium">{board.name}</span>
                  <Badge variant="outline" className="tabular-nums">
                    {board.taskPrefix}
                  </Badge>
                </Link>
              </li>
            ))}
          </ul>
        ) : null}
      </CardContent>
    </Card>
  )
}
