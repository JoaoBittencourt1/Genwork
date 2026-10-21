import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArchiveIcon, ChevronDownIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import type { DirectoryUser } from '@/shared/datasource/users'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { Field, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Textarea } from '@/shared/ui/textarea'
import { archiveTask, createTask, deleteTask, updateTask } from '../datasource/tasksDatasource'
import { toggleValue } from '../domain/filters'
import type { GentasksAbilities } from '../domain/permissions'
import {
  buildTaskPatch,
  draftFromTask,
  EMPTY_TASK_DRAFT,
  isDraftTitleValid,
  isPatchEmpty,
  newTaskFromDraft,
  type TaskDraft,
} from '../domain/taskPatch'
import { URGENCY_LEVELS, type Label, type Task, type Urgency } from '../domain/types'
import { urgencyTranslationKey } from '../domain/urgency'
import { LabelChip } from './LabelChip'
import { gentasksQueryKeys } from './queryKeys'

const NO_ASSIGNEE = 'none'

export type TaskDialogMode = { kind: 'create'; columnToken: string } | { kind: 'edit'; task: Task }

type TaskDialogProps = {
  boardToken: string
  mode: TaskDialogMode | null
  labels: Label[]
  users: DirectoryUser[] | undefined
  abilities: GentasksAbilities
  onClose: () => void
}

export function TaskDialog({ mode, onClose, ...props }: TaskDialogProps) {
  return (
    <Dialog open={mode !== null} onOpenChange={(open) => (open ? undefined : onClose())}>
      <DialogContent className="sm:max-w-lg">
        {mode ? <TaskForm key={mode.kind === 'edit' ? mode.task.token : mode.columnToken} mode={mode} onClose={onClose} {...props} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function TaskForm({ boardToken, mode, labels, users, abilities, onClose }: TaskDialogProps & { mode: TaskDialogMode }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TaskDraft>(mode.kind === 'edit' ? draftFromTask(mode.task) : EMPTY_TASK_DRAFT)
  const [confirmingDelete, setConfirmingDelete] = useState(false)
  const readOnly = !abilities.canWriteTasks

  const finish = async () => {
    await queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.board(boardToken) })
    onClose()
  }
  const save = useMutation({
    mutationFn: async () => {
      if (mode.kind === 'create') return createTask(boardToken, newTaskFromDraft(draft, mode.columnToken))
      const patch = buildTaskPatch(mode.task, draft)
      return isPatchEmpty(patch) ? mode.task : updateTask(mode.task.token, patch)
    },
    onSuccess: finish,
  })
  const archive = useMutation({ mutationFn: (task: Task) => archiveTask(task.token), onSuccess: finish })
  const remove = useMutation({ mutationFn: (task: Task) => deleteTask(task.token), onSuccess: finish })
  const error = save.error ?? archive.error ?? remove.error
  const titleError = describeFieldError(save.error, 'title')
  const assigneeError = describeFieldError(save.error, 'assigneeToken')
  const selectedLabels = labels.filter((label) => draft.labelTokens.includes(label.token))

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{mode.kind === 'create' ? t('gentasks.task.newTitle') : t('gentasks.task.editTitle')}</DialogTitle>
      </DialogHeader>
      {titleError || assigneeError ? null : <ErrorAlert error={error} />}
      <Field data-invalid={Boolean(titleError)}>
        <FieldLabel htmlFor="task-title">{t('gentasks.task.title')}</FieldLabel>
        <Input
          id="task-title"
          value={draft.title}
          maxLength={200}
          autoFocus
          required
          disabled={readOnly}
          aria-invalid={Boolean(titleError)}
          onChange={(event) => setDraft({ ...draft, title: event.target.value })}
        />
        {titleError ? <FieldError>{titleError}</FieldError> : null}
      </Field>
      <Field>
        <FieldLabel htmlFor="task-description">{t('gentasks.task.description')}</FieldLabel>
        <Textarea
          id="task-description"
          value={draft.description}
          maxLength={10000}
          rows={5}
          disabled={readOnly}
          onChange={(event) => setDraft({ ...draft, description: event.target.value })}
        />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field data-invalid={Boolean(assigneeError)}>
          <FieldLabel>{t('gentasks.task.assignee')}</FieldLabel>
          <Select
            value={draft.assigneeToken ?? NO_ASSIGNEE}
            disabled={readOnly}
            onValueChange={(value) => setDraft({ ...draft, assigneeToken: value === NO_ASSIGNEE ? null : value })}
          >
            <SelectTrigger className="w-full" aria-label={t('gentasks.task.assignee')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_ASSIGNEE}>{t('gentasks.task.unassigned')}</SelectItem>
              {(users ?? []).map((user) => (
                <SelectItem key={user.token} value={user.token}>
                  {user.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {assigneeError ? <FieldError>{assigneeError}</FieldError> : null}
        </Field>
        <Field>
          <FieldLabel>{t('gentasks.task.urgency')}</FieldLabel>
          <Select value={draft.urgency} disabled={readOnly} onValueChange={(value) => setDraft({ ...draft, urgency: value as Urgency })}>
            <SelectTrigger className="w-full" aria-label={t('gentasks.task.urgency')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {URGENCY_LEVELS.map((urgency) => (
                <SelectItem key={urgency} value={urgency}>
                  {t(urgencyTranslationKey(urgency))}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      </div>
      <Field>
        <FieldLabel>{t('gentasks.task.labels')}</FieldLabel>
        <DropdownMenu>
          <DropdownMenuTrigger asChild disabled={readOnly || labels.length === 0}>
            <Button type="button" variant="outline" className="h-auto min-h-9 justify-between">
              <span className="flex flex-wrap gap-1">
                {selectedLabels.length > 0
                  ? selectedLabels.map((label) => <LabelChip key={label.token} label={label} />)
                  : t(labels.length === 0 ? 'gentasks.task.noLabelsOnBoard' : 'gentasks.task.noLabels')}
              </span>
              <ChevronDownIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {labels.map((label) => (
              <DropdownMenuCheckboxItem
                key={label.token}
                checked={draft.labelTokens.includes(label.token)}
                onSelect={(event) => event.preventDefault()}
                onCheckedChange={() => setDraft({ ...draft, labelTokens: toggleValue(draft.labelTokens, label.token) })}
              >
                <LabelChip label={label} />
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </Field>
      <DialogFooter className="gap-2 sm:justify-between">
        <div className="flex gap-2">
          {mode.kind === 'edit' && abilities.canWriteTasks ? (
            <Button type="button" variant="outline" disabled={archive.isPending} onClick={() => archive.mutate(mode.task)}>
              <ArchiveIcon />
              {t('gentasks.task.archive')}
            </Button>
          ) : null}
          {mode.kind === 'edit' && abilities.canDeleteTasks ? (
            <Button type="button" variant="destructive" disabled={remove.isPending} onClick={() => setConfirmingDelete(true)}>
              <Trash2Icon />
              {t('common.delete')}
            </Button>
          ) : null}
        </div>
        {readOnly ? null : (
          <Button type="submit" disabled={save.isPending || !isDraftTitleValid(draft)}>
            {save.isPending ? t('common.saving') : mode.kind === 'create' ? t('gentasks.task.create') : t('common.save')}
          </Button>
        )}
      </DialogFooter>
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
