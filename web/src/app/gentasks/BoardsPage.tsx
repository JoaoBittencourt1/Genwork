import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { KanbanSquareIcon, MoreHorizontalIcon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { PageHeader } from '@/shared/components/PageHeader'
import { Button } from '@/shared/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/shared/ui/dropdown-menu'
import { Spinner } from '@/shared/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { BoardBreadcrumb } from './components/BoardBreadcrumb'
import { GentasksNoPermission } from './components/GentasksNoPermission'
import { NameDialog } from './components/NameDialog'
import { gentasksQueryKeys } from './components/queryKeys'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { createBoard, deleteBoard, listBoards } from './datasource/boardsDatasource'
import { MAX_BOARD_NAME_LENGTH } from './domain/boardSettings'
import type { BoardSummary } from './domain/types'

export function BoardsPage() {
  const { t, i18n } = useTranslation()
  const abilities = useGentasksAbilities()
  const queryClient = useQueryClient()
  const boards = useQuery({ queryKey: gentasksQueryKeys.boards, queryFn: listBoards, enabled: abilities.canRead })
  const [creating, setCreating] = useState(false)
  const [deleting, setDeleting] = useState<BoardSummary | null>(null)
  const formatDate = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' })
  const refresh = () => queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.boards })

  const create = useMutation({
    mutationFn: createBoard,
    onSuccess: async () => {
      setCreating(false)
      await refresh()
    },
  })
  const remove = useMutation({ mutationFn: (board: BoardSummary) => deleteBoard(board.token), onSuccess: refresh })

  if (!abilities.canRead) return <GentasksNoPermission />

  return (
    <>
      <PageHeader
        actions={
          abilities.canManageBoards ? (
            <Button
              size="sm"
              onClick={() => {
                create.reset()
                setCreating(true)
              }}
            >
              <PlusIcon />
              {t('gentasks.boards.create')}
            </Button>
          ) : null
        }
      >
        <BoardBreadcrumb />
      </PageHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="empty:hidden px-6 pt-4">
          <ErrorAlert error={boards.error ?? remove.error} />
        </div>
        {boards.isPending ? <Spinner className="m-6" /> : null}
        {boards.data?.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-4 py-24 text-center">
            <KanbanSquareIcon className="size-8 text-muted-foreground" aria-hidden />
            <p className="text-sm text-muted-foreground">{t('gentasks.boards.empty')}</p>
          </div>
        ) : null}
        {boards.data && boards.data.length > 0 ? (
          <Table aria-label={t('gentasks.boards.title')}>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="pl-6">{t('gentasks.boards.name')}</TableHead>
                <TableHead className="w-28">{t('gentasks.settings.taskPrefix')}</TableHead>
                <TableHead className="hidden w-36 sm:table-cell">{t('gentasks.boards.updated')}</TableHead>
                <TableHead className="w-12 pr-6">
                  <span className="sr-only">{t('gentasks.boards.actionsColumn')}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {boards.data.map((board) => (
                <TableRow key={board.token} className="relative h-11">
                  <TableCell className="pl-6">
                    <span className="flex min-w-0 items-center gap-2">
                      <KanbanSquareIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                      <Link
                        to="/tasks/$boardToken"
                        params={{ boardToken: board.token }}
                        className="truncate font-medium after:absolute after:inset-0"
                      >
                        {board.name}
                      </Link>
                    </span>
                  </TableCell>
                  <TableCell className="text-muted-foreground tabular-nums">{board.taskPrefix}</TableCell>
                  <TableCell className="hidden text-muted-foreground sm:table-cell">{formatDate.format(new Date(board.updatedAt))}</TableCell>
                  <TableCell className="relative z-10 pr-6">
                    {abilities.canManageBoards ? (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon-sm" aria-label={t('gentasks.boards.actions', { name: board.name })}>
                            <MoreHorizontalIcon />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem asChild>
                            <Link to="/tasks/$boardToken/settings" params={{ boardToken: board.token }}>
                              {t('gentasks.settings.title')}
                            </Link>
                          </DropdownMenuItem>
                          <DropdownMenuItem variant="destructive" onSelect={() => setDeleting(board)}>
                            {t('common.delete')}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    ) : null}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : null}
      </div>
      <NameDialog
        open={creating}
        title={t('gentasks.boards.createTitle')}
        label={t('gentasks.boards.name')}
        initialName=""
        submitLabel={t('gentasks.boards.create')}
        maxLength={MAX_BOARD_NAME_LENGTH}
        isSaving={create.isPending}
        error={create.error}
        onSubmit={(name) => create.mutate(name)}
        onOpenChange={(open) => (open ? undefined : setCreating(false))}
      />
      <ConfirmDialog
        open={deleting !== null}
        title={t('gentasks.boards.deleteTitle')}
        description={t('gentasks.boards.deleteDescription', { name: deleting?.name ?? '' })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleting && remove.mutate(deleting)}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
      />
    </>
  )
}
