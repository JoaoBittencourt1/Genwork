import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'

type NameDialogProps = {
  open: boolean
  title: string
  label: string
  initialName: string
  submitLabel: string
  maxLength: number
  isSaving: boolean
  error: unknown
  onSubmit: (name: string) => void
  onOpenChange: (open: boolean) => void
}

// NameDialog asks for one name; it serves boards and columns alike.
export function NameDialog(props: NameDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>{props.open ? <NameForm {...props} /> : null}</DialogContent>
    </Dialog>
  )
}

function NameForm({ title, label, initialName, submitLabel, maxLength, isSaving, error, onSubmit }: NameDialogProps) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const [name, setName] = useState(initialName)
  const nameError = describeFieldError(error, 'name')

  function submit(event: FormEvent) {
    event.preventDefault()
    onSubmit(name)
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
      </DialogHeader>
      {nameError ? null : <ErrorAlert error={error} />}
      <Field data-invalid={Boolean(nameError)}>
        <FieldLabel htmlFor="name-dialog-input">{label}</FieldLabel>
        <Input
          id="name-dialog-input"
          value={name}
          maxLength={maxLength}
          autoFocus
          required
          aria-invalid={Boolean(nameError)}
          onChange={(event) => setName(event.target.value)}
        />
        {nameError ? <FieldError>{nameError}</FieldError> : null}
      </Field>
      <DialogFooter>
        <Button type="submit" disabled={isSaving || name.trim() === ''}>
          {isSaving ? t('common.saving') : submitLabel}
        </Button>
      </DialogFooter>
    </form>
  )
}
