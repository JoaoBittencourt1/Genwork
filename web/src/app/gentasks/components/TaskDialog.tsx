import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArchiveIcon, ChevronDownIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import type { DirectoryUser } from '@/shared/datasource/users'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/shared/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { FieldError } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Textarea } from '@/shared/ui/textarea'
import { archiveTask, createTask, deleteTask, moveTask, updateTask } from '../datasource/tasksDatasource'
import { resolveColumnTitle } from '../domain/columnName'
import { toggleValue } from '../domain/filters'
import type { GentasksAbilities } from '../domain/permissions'
import {
  buildTaskPatch,
  draftFromTask,
  emptyTaskDraft,
  isDraftTitleValid,
  newTaskFromDraft,
  resolveColumnChange,
  type TaskDraft,
} from '../domain/taskPatch'
import { isPatchEmpty } from '../domain/patch'
import { taskIdentifier } from '../domain/taskPrefix'
import { URGENCY_LEVELS, type BoardView, type Task, type Urgency } from '../domain/types'
import { urgencyTranslationKey } from '../domain/urgency'
import { AssigneeAvatar } from './AssigneeAvatar'
import { ColumnStatusIcon } from './ColumnStatusIcon'
import { LabelChip } from './LabelChip'
import { gentasksQueryKeys } from './queryKeys'
import { UrgencyIcon } from './UrgencyIcon'

const NO_ASSIGNEE = 'none'

export type TaskDialogMode = { kind: 'create'; columnToken: string } | { kind: 'edit'; task: Task }

type TaskDialogProps = {
  board: BoardView
  mode: TaskDialogMode | null
  users: DirectoryUser[] | undefined
  abilities: GentasksAbilities
  onClose: () => void
}

export function TaskDialog({ mode, onClose, ...props }: TaskDialogProps) {
  return (
    <Dialog open={mode !== null} onOpenChange={(open) => (open ? undefined : onClose())}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-4xl">
        {mode ? <TaskForm key={mode.kind === 'edit' ? mode.task.token : mode.columnToken} mode={mode} onClose={onClose} {...props} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function TaskForm({ board, mode, users, abilities, onClose }: TaskDialogProps & { mode: TaskDialogMode }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TaskDraft>(mode.kind === 'edit' ? draftFromTask(mode.task) : emptyTaskDraft(mode.columnToken))
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const readOnly = !abilities.canWriteTasks

  const finish = async () => {
    await queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.board(board.token) })
    onClose()
  }
  const save = useMutation({
    mutationFn: async () => {
      if (mode.kind === 'create') return createTask(board.token, newTaskFromDraft(draft))
      const patch = buildTaskPatch(mode.task, draft)
      const updated = isPatchEmpty(patch) ? mode.task : await updateTask(mode.task.token, patch)
      const move = resolveColumnChange(board, mode.task, draft)
      if (move) await moveTask(move.taskToken, move.toColumnToken, move.toPosition)
      return updated
    },
    onSuccess: finish,
  })
  const archive = useMutation({ mutationFn: (task: Task) => archiveTask(task.token), onSuccess: finish })
  const remove = useMutation({ mutationFn: (task: Task) => deleteTask(task.token), onSuccess: finish })
  const error = save.error ?? archive.error ?? remove.error
  const titleError = describeFieldError(save.error, 'title')
  const assigneeError = describeFieldError(save.error, 'assigneeToken')
  const heading = mode.kind === 'edit' ? taskIdentifier(board.taskPrefix, mode.task.number) : t('gentasks.task.newTitle')

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  return (
    <form onSubmit={submit} className="flex max-h-[85svh] flex-col">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b px-4 pr-12 text-sm">
        <span className="text-muted-foreground">{board.name}</span>
        <span className="text-muted-foreground">›</span>
        <DialogTitle className="text-sm font-medium tabular-nums">{heading}</DialogTitle>
        <DialogDescription className="sr-only">{t('gentasks.task.dialogDescription')}</DialogDescription>
      </div>
      <div className="grid min-h-0 flex-1 overflow-y-auto md:grid-cols-[minmax(0,1fr)_16rem]">
        <div className="flex min-w-0 flex-col gap-2 p-6">
          {titleError || assigneeError ? null : <ErrorAlert error={error} />}
          <Input
            value={draft.title}
            maxLength={200}
            autoFocus
            required
            disabled={readOnly}
            placeholder={t('gentasks.task.titlePlaceholder')}
            aria-label={t('gentasks.task.title')}
            aria-invalid={Boolean(titleError)}
            className="h-auto border-none bg-transparent px-0 text-xl font-semibold shadow-none focus-visible:ring-0 md:text-xl dark:bg-transparent"
            onChange={(event) => setDraft({ ...draft, title: event.target.value })}
          />
          {titleError ? <FieldError>{titleError}</FieldError> : null}
          <Textarea
            value={draft.description}
            maxLength={10000}
            rows={10}
            disabled={readOnly}
            placeholder={t('gentasks.task.descriptionPlaceholder')}
            aria-label={t('gentasks.task.description')}
            className="min-h-48 resize-none border-none bg-transparent px-0 shadow-none focus-visible:ring-0 dark:bg-transparent"
            onChange={(event) => setDraft({ ...draft, description: event.target.value })}
          />
        </div>
        <TaskProperties board={board} draft={draft} users={users} readOnly={readOnly} assigneeError={assigneeError} onChange={setDraft} />
      </div>
      <div className="flex shrink-0 items-center gap-2 border-t px-4 py-3">
        {mode.kind === 'edit' && abilities.canWriteTasks ? (
          <Button type="button" variant="ghost" size="sm" disabled={archive.isPending} onClick={() => archive.mutate(mode.task)}>
            <ArchiveIcon />
            {t('gentasks.task.archive')}
          </Button>
        ) : null}
        {mode.kind === 'edit' && abilities.canDeleteTasks ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="text-destructive hover:text-destructive"
            disabled={remove.isPending}
            onClick={() => setConfirmingDelete(true)}
          >
            <Trash2Icon />
            {t('common.delete')}
          </Button>
        ) : null}
        {readOnly ? null : (
          <Button type="submit" size="sm" className="ml-auto" disabled={save.isPending || !isDraftTitleValid(draft)}>
            {save.isPending ? t('common.saving') : mode.kind === 'create' ? t('gentasks.task.create') : t('common.save')}
          </Button>
        )}
      </div>
      {mode.kind === 'edit' ? (
        <ConfirmDialog
          open={confirmingDelete}
          title={t('gentasks.task.deleteTitle')}
          description={t('gentasks.task.deleteDescription', { title: mode.task.title })}
          confirmLabel={t('common.delete')}
          onConfirm={() => remove.mutate(mode.task)}
          onOpenChange={setConfirmingDelete}
        />
      ) : null}
    </form>
  )
}

type TaskPropertiesProps = {
  board: BoardView
  draft: TaskDraft
  users: DirectoryUser[] | undefined
  readOnly: boolean
  assigneeError: string | undefined
  onChange: (draft: TaskDraft) => void
}

function TaskProperties({ board, draft, users, readOnly, assigneeError, onChange }: TaskPropertiesProps) {
  const { t } = useTranslation()
  const selectedLabels = board.labels.filter((label) => draft.labelTokens.includes(label.token))

  return (
    <aside className="flex flex-col gap-1 border-t bg-muted/30 p-4 md:border-t-0 md:border-l">
      <p className="px-2 pb-2 text-xs font-medium text-muted-foreground">{t('gentasks.task.properties')}</p>
      <Property label={t('gentasks.task.status')}>
        <Select value={draft.columnToken} disabled={readOnly} onValueChange={(columnToken) => onChange({ ...draft, columnToken })}>
          <PropertyTrigger label={t('gentasks.task.status')} />
          <SelectContent>
            {board.columns.map((column) => (
              <SelectItem key={column.token} value={column.token}>
                <ColumnStatusIcon column={column} />
                {resolveColumnTitle(column, t)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Property>
      <Property label={t('gentasks.task.urgency')}>
        <Select value={draft.urgency} disabled={readOnly} onValueChange={(value) => onChange({ ...draft, urgency: value as Urgency })}>
          <PropertyTrigger label={t('gentasks.task.urgency')} />
          <SelectContent>
            {URGENCY_LEVELS.map((urgency) => (
              <SelectItem key={urgency} value={urgency}>
                <UrgencyIcon urgency={urgency} />
                {t(urgencyTranslationKey(urgency))}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Property>
      <Property label={t('gentasks.task.assignee')}>
        <Select
          value={draft.assigneeToken ?? NO_ASSIGNEE}
          disabled={readOnly}
          onValueChange={(value) => onChange({ ...draft, assigneeToken: value === NO_ASSIGNEE ? null : value })}
        >
          <PropertyTrigger label={t('gentasks.task.assignee')} invalid={Boolean(assigneeError)} />
          <SelectContent>
            <SelectItem value={NO_ASSIGNEE}>
              <AssigneeAvatar assignee={undefined} unassignedLabel={t('gentasks.task.unassigned')} />
              {t('gentasks.task.unassigned')}
            </SelectItem>
            {(users ?? []).map((user) => (
              <SelectItem key={user.token} value={user.token}>
                <AssigneeAvatar assignee={user} unassignedLabel={t('gentasks.task.unassigned')} />
                {user.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Property>
      {assigneeError ? <FieldError className="px-2">{assigneeError}</FieldError> : null}
      <Property label={t('gentasks.task.labels')}>
        <DropdownMenu>
          <DropdownMenuTrigger asChild disabled={readOnly || board.labels.length === 0}>
            <Button type="button" variant="ghost" size="sm" className="h-auto min-h-8 w-full justify-between px-2 font-normal">
              <span className="flex min-w-0 flex-wrap gap-1">
                {selectedLabels.length > 0 ? (
                  selectedLabels.map((label) => <LabelChip key={label.token} label={label} />)
                ) : (
                  <span className="text-muted-foreground">
                    {t(board.labels.length === 0 ? 'gentasks.task.noLabelsOnBoard' : 'gentasks.task.noLabels')}
                  </span>
                )}
              </span>
              <ChevronDownIcon className="text-muted-foreground" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {board.labels.map((label) => (
              <DropdownMenuCheckboxItem
                key={label.token}
                checked={draft.labelTokens.includes(label.token)}
                onSelect={(event) => event.preventDefault()}
                onCheckedChange={() => onChange({ ...draft, labelTokens: toggleValue(draft.labelTokens, label.token) })}
              >
                <LabelChip label={label} />
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </Property>
    </aside>
  )
}

function Property({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[5.5rem_minmax(0,1fr)] items-center gap-1">
      <span className="px-2 text-xs text-muted-foreground">{label}</span>
      {children}
    </div>
  )
}

function PropertyTrigger({ label, invalid }: { label: string; invalid?: boolean }) {
  return (
    <SelectTrigger
      size="sm"
      aria-label={label}
      aria-invalid={invalid}
      className="w-full border-none bg-transparent px-2 shadow-none hover:bg-accent dark:bg-transparent dark:hover:bg-accent"
    >
      <SelectValue />
    </SelectTrigger>
  )
}
