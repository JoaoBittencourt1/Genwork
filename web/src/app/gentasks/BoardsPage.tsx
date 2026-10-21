import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { MoreHorizontalIcon, PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Button } from '@/shared/ui/button'
import { Card, CardAction, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/shared/ui/dropdown-menu'
import { Spinner } from '@/shared/ui/spinner'
import { NameDialog } from './components/NameDialog'
import { gentasksQueryKeys } from './components/queryKeys'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { createBoard, deleteBoard, listBoards, renameBoard } from './datasource/boardsDatasource'
import type { BoardSummary } from './domain/types'

type BoardNameEditing = { kind: 'create' } | { kind: 'rename'; board: BoardSummary }

export function BoardsPage() {
  const { t, i18n } = useTranslation()
  const abilities = useGentasksAbilities()
  const queryClient = useQueryClient()
  const boards = useQuery({ queryKey: gentasksQueryKeys.boards, queryFn: listBoards, enabled: abilities.canRead })
  const [editing, setEditing] = useState<BoardNameEditing | null>(null)
  const [deleting, setDeleting] = useState<BoardSummary | null>(null)
  const formatDate = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' })
  const refresh = () => queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.boards })

  const saveName = useMutation({
    mutationFn: (name: string) => (editing?.kind === 'rename' ? renameBoard(editing.board.token, name) : createBoard(name)),
    onSuccess: async () => {
      setEditing(null)
      await refresh()
    },
  })
  const remove = useMutation({ mutationFn: (board: BoardSummary) => deleteBoard(board.token), onSuccess: refresh })

  if (!abilities.canRead) {
    return (
      <section className="space-y-2">
        <h1 className="text-2xl font-semibold tracking-tight">{t('gentasks.noPermissionTitle')}</h1>
        <p className="text-muted-foreground">{t('gentasks.noPermissionDescription')}</p>
      </section>
    )
  }

  function openEditing(next: BoardNameEditing) {
    saveName.reset()
    setEditing(next)
  }

  return (
    <section className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold tracking-tight">{t('gentasks.boards.title')}</h1>
        {abilities.canManageBoards ? (
          <Button onClick={() => openEditing({ kind: 'create' })}>
            <PlusIcon />
            {t('gentasks.boards.create')}
          </Button>
        ) : null}
      </div>
      <ErrorAlert error={boards.error ?? remove.error} />
      {boards.isPending ? <Spinner /> : null}
      {boards.data?.length === 0 ? <p className="text-sm text-muted-foreground">{t('gentasks.boards.empty')}</p> : null}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {boards.data?.map((board) => (
          <Card key={board.token} className="relative transition-colors hover:border-primary">
            <CardHeader>
              <CardTitle>
                <Link to="/tasks/$boardToken" params={{ boardToken: board.token }} className="after:absolute after:inset-0">
                  {board.name}
                </Link>
              </CardTitle>
              <CardDescription>{t('gentasks.boards.updatedAt', { date: formatDate.format(new Date(board.updatedAt)) })}</CardDescription>
              {abilities.canManageBoards ? (
                <CardAction className="relative z-10">
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon-sm" aria-label={t('gentasks.boards.actions', { name: board.name })}>
                        <MoreHorizontalIcon />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onSelect={() => openEditing({ kind: 'rename', board })}>{t('gentasks.boards.rename')}</DropdownMenuItem>
                      <DropdownMenuItem variant="destructive" onSelect={() => setDeleting(board)}>
                        {t('common.delete')}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </CardAction>
              ) : null}
            </CardHeader>
          </Card>
        ))}
      </div>
      <NameDialog
        open={editing !== null}
        title={editing?.kind === 'rename' ? t('gentasks.boards.renameTitle') : t('gentasks.boards.createTitle')}
        label={t('gentasks.boards.name')}
        initialName={editing?.kind === 'rename' ? editing.board.name : ''}
        submitLabel={editing?.kind === 'rename' ? t('common.save') : t('gentasks.boards.create')}
        maxLength={80}
        isSaving={saveName.isPending}
        error={saveName.error}
        onSubmit={(name) => saveName.mutate(name)}
        onOpenChange={(open) => (open ? undefined : setEditing(null))}
      />
      <ConfirmDialog
        open={deleting !== null}
        title={t('gentasks.boards.deleteTitle')}
        description={t('gentasks.boards.deleteDescription', { name: deleting?.name ?? '' })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleting && remove.mutate(deleting)}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
      />
    </section>
  )
}
