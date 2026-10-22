import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { PageContent } from '@/shared/components/PageContent'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageSection } from '@/shared/components/PageSection'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { BoardBreadcrumb } from './components/BoardBreadcrumb'
import { GentasksNoPermission } from './components/GentasksNoPermission'
import { GithubSettingsSection } from './components/GithubSettingsSection'
import { LabelManager } from './components/LabelManager'
import { gentasksQueryKeys } from './components/queryKeys'
import { useGentasksAbilities } from './components/useGentasksAbilities'
import { deleteBoard, readBoard, updateBoard } from './datasource/boardsDatasource'
import {
  buildBoardPatch,
  draftFromBoard,
  isBoardSettingsDraftValid,
  MAX_BOARD_NAME_LENGTH,
  type BoardSettingsDraft,
} from './domain/boardSettings'
import { isPatchEmpty } from './domain/patch'
import { isValidTaskPrefix, MAX_TASK_PREFIX_LENGTH, normalizeTaskPrefix, taskIdentifier } from './domain/taskPrefix'
import type { BoardView } from './domain/types'

export function BoardSettingsPage({ boardToken }: { boardToken: string }) {
  const { t } = useTranslation()
  const abilities = useGentasksAbilities()
  const board = useQuery({
    queryKey: gentasksQueryKeys.board(boardToken),
    queryFn: () => readBoard(boardToken),
    enabled: abilities.canManageBoards,
  })

  if (!abilities.canManageBoards) return <GentasksNoPermission />

  return (
    <>
      <PageHeader>
        <BoardBreadcrumb board={board.data} current={t('gentasks.settings.title')} />
      </PageHeader>
      <PageContent className="max-w-3xl">
        {board.isPending ? <Spinner /> : null}
        {board.isError ? <ErrorAlert error={board.error} /> : null}
        {board.data ? (
          <div className="flex flex-col gap-10">
            <PageSection title={t('gentasks.settings.general')} description={t('gentasks.settings.generalDescription')}>
              <GeneralSettingsForm key={board.data.updatedAt} board={board.data} />
            </PageSection>
            <PageSection title={t('gentasks.labels.title')} description={t('gentasks.labels.description')}>
              <LabelManager boardToken={boardToken} labels={board.data.labels} />
            </PageSection>
            <PageSection title={t('gentasks.github.settings.title')} description={t('gentasks.github.settings.description')}>
              <GithubSettingsSection board={board.data} canManageGithub={abilities.canManageGithub} />
            </PageSection>
            <PageSection title={t('gentasks.settings.dangerZone')} description={t('gentasks.settings.dangerZoneDescription')}>
              <DeleteBoardButton board={board.data} />
            </PageSection>
          </div>
        ) : null}
      </PageContent>
    </>
  )
}

function GeneralSettingsForm({ board }: { board: BoardView }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<BoardSettingsDraft>(draftFromBoard(board))
  const patch = buildBoardPatch(board, draft)
  const save = useMutation({
    mutationFn: () => updateBoard(board.token, patch),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.board(board.token) }),
        queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.boards }),
      ])
      toast.success(t('gentasks.settings.saved'))
    },
  })
  const nameError = describeFieldError(save.error, 'name')
  const prefixError = describeFieldError(save.error, 'taskPrefix')
  const normalizedPrefix = normalizeTaskPrefix(draft.taskPrefix)
  const previewPrefix = isValidTaskPrefix(normalizedPrefix) ? normalizedPrefix : board.taskPrefix

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-5">
      {nameError || prefixError ? null : <ErrorAlert error={save.error} />}
      <Field data-invalid={Boolean(nameError)}>
        <FieldLabel htmlFor="board-name">{t('gentasks.boards.name')}</FieldLabel>
        <Input
          id="board-name"
          value={draft.name}
          maxLength={MAX_BOARD_NAME_LENGTH}
          required
          aria-invalid={Boolean(nameError)}
          onChange={(event) => setDraft({ ...draft, name: event.target.value })}
        />
        {nameError ? <FieldError>{nameError}</FieldError> : null}
      </Field>
      <Field data-invalid={Boolean(prefixError)}>
        <FieldLabel htmlFor="board-task-prefix">{t('gentasks.settings.taskPrefix')}</FieldLabel>
        <Input
          id="board-task-prefix"
          value={draft.taskPrefix}
          maxLength={MAX_TASK_PREFIX_LENGTH}
          required
          className="w-40 uppercase"
          aria-invalid={Boolean(prefixError)}
          onChange={(event) => setDraft({ ...draft, taskPrefix: event.target.value })}
        />
        <FieldDescription>
          {t('gentasks.settings.taskPrefixDescription', { example: taskIdentifier(previewPrefix, 123) })}
        </FieldDescription>
        {prefixError ? <FieldError>{prefixError}</FieldError> : null}
      </Field>
      <div>
        <Button type="submit" size="sm" disabled={save.isPending || isPatchEmpty(patch) || !isBoardSettingsDraftValid(draft)}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </Button>
      </div>
    </form>
  )
}

function DeleteBoardButton({ board }: { board: BoardView }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [confirming, setConfirming] = useState(false)
  const remove = useMutation({
    mutationFn: () => deleteBoard(board.token),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.boards })
      await navigate({ to: '/tasks' })
    },
  })

  return (
    <div className="flex flex-col gap-3">
      <ErrorAlert error={remove.error} />
      <div>
        <Button variant="destructive" size="sm" disabled={remove.isPending} onClick={() => setConfirming(true)}>
          {t('gentasks.settings.deleteBoard')}
        </Button>
      </div>
      <ConfirmDialog
        open={confirming}
        title={t('gentasks.boards.deleteTitle')}
        description={t('gentasks.boards.deleteDescription', { name: board.name })}
        confirmLabel={t('common.delete')}
        onConfirm={() => remove.mutate()}
        onOpenChange={setConfirming}
      />
    </div>
  )
}
