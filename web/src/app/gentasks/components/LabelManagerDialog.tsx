import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { createLabel, deleteLabel, updateLabel } from '../datasource/labelsDatasource'
import { labelColorTranslationKey, labelColorVariable } from '../domain/labelPalette'
import { LABEL_COLORS, type Label, type LabelColor, type LabelPatch } from '../domain/types'
import { gentasksQueryKeys } from './queryKeys'

type LabelManagerDialogProps = {
  boardToken: string
  labels: Label[]
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function LabelManagerDialog({ boardToken, labels, open, onOpenChange }: LabelManagerDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const refresh = () => queryClient.invalidateQueries({ queryKey: gentasksQueryKeys.board(boardToken) })
  const update = useMutation({
    mutationFn: ({ label, patch }: { label: Label; patch: LabelPatch }) => updateLabel(label.token, patch),
    onSettled: refresh,
  })
  const remove = useMutation({ mutationFn: (label: Label) => deleteLabel(label.token), onSuccess: refresh })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('gentasks.labels.title')}</DialogTitle>
          <DialogDescription>{t('gentasks.labels.description')}</DialogDescription>
        </DialogHeader>
        <NewLabelForm boardToken={boardToken} onCreated={refresh} />
        <ErrorAlert error={update.error ?? remove.error} />
        {labels.length === 0 ? <p className="text-sm text-muted-foreground">{t('gentasks.labels.empty')}</p> : null}
        <ul className="flex flex-col gap-2">
          {labels.map((label) => (
            <li key={label.token} className="flex items-center gap-2">
              <Input
                defaultValue={label.name}
                maxLength={40}
                aria-label={t('gentasks.labels.name')}
                onBlur={(event) => {
                  const name = event.target.value.trim()
                  if (name !== '' && name !== label.name) update.mutate({ label, patch: { name } })
                }}
              />
              <ColorSelect value={label.color} onChange={(color) => update.mutate({ label, patch: { color } })} />
              <Button
                variant="ghost"
                size="icon"
                aria-label={t('gentasks.labels.delete', { name: label.name })}
                disabled={remove.isPending}
                onClick={() => remove.mutate(label)}
              >
                <Trash2Icon />
              </Button>
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  )
}

function NewLabelForm({ boardToken, onCreated }: { boardToken: string; onCreated: () => Promise<void> }) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [color, setColor] = useState<LabelColor>('gray')
  const create = useMutation({
    mutationFn: () => createLabel(boardToken, name, color),
    onSuccess: async () => {
      setName('')
      await onCreated()
    },
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    create.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-2">
      <ErrorAlert error={create.error} />
      <div className="flex items-center gap-2">
        <Input
          value={name}
          maxLength={40}
          placeholder={t('gentasks.labels.newPlaceholder')}
          aria-label={t('gentasks.labels.name')}
          onChange={(event) => setName(event.target.value)}
        />
        <ColorSelect value={color} onChange={setColor} />
        <Button type="submit" disabled={create.isPending || name.trim() === ''}>
          {t('gentasks.labels.create')}
        </Button>
      </div>
    </form>
  )
}

function ColorSelect({ value, onChange }: { value: LabelColor; onChange: (color: LabelColor) => void }) {
  const { t } = useTranslation()
  return (
    <Select value={value} onValueChange={(color) => onChange(color as LabelColor)}>
      <SelectTrigger className="w-36 shrink-0" aria-label={t('gentasks.labels.color')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {LABEL_COLORS.map((color) => (
          <SelectItem key={color} value={color}>
            <span className="size-3 rounded-full" style={{ backgroundColor: labelColorVariable(color) }} />
            {t(labelColorTranslationKey(color))}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
