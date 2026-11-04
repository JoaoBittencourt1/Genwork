import { useState, type FocusEvent, type KeyboardEvent, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { MarkdownContent } from '@/shared/components/MarkdownContent'
import { Textarea } from '@/shared/ui/textarea'
import { isDescriptionBlank, MAX_DESCRIPTION_LENGTH } from '../domain/taskDescription'

type TaskDescriptionFieldProps = {
  value: string
  readOnly: boolean
  onChange: (description: string) => void
}

const VIEW_CLASS = 'min-h-48 rounded-md py-2 text-left'

export function TaskDescriptionField({ value, readOnly, onChange }: TaskDescriptionFieldProps) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const rendered = isDescriptionBlank(value) ? (
    <p className="text-sm text-muted-foreground">{t('gentasks.task.descriptionPlaceholder')}</p>
  ) : (
    <MarkdownContent markdown={value} />
  )

  if (readOnly) return <div className={VIEW_CLASS}>{rendered}</div>

  if (editing) {
    return (
      <Textarea
        value={value}
        maxLength={MAX_DESCRIPTION_LENGTH}
        rows={10}
        autoFocus
        placeholder={t('gentasks.task.descriptionPlaceholder')}
        aria-label={t('gentasks.task.description')}
        className="min-h-48 resize-none border-none bg-transparent px-0 shadow-none focus-visible:ring-0 dark:bg-transparent"
        onFocus={placeCaretAtEnd}
        onBlur={() => setEditing(false)}
        onChange={(event) => onChange(event.target.value)}
      />
    )
  }

  function startEditingOnClick(event: MouseEvent<HTMLDivElement>) {
    if (!(event.target instanceof Element && event.target.closest('a'))) setEditing(true)
  }

  function startEditingOnKey(event: KeyboardEvent<HTMLDivElement>) {
    if (event.target !== event.currentTarget || (event.key !== 'Enter' && event.key !== ' ')) return
    event.preventDefault()
    setEditing(true)
  }

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={t('gentasks.task.editDescription')}
      className={`${VIEW_CLASS} cursor-text focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none`}
      onClick={startEditingOnClick}
      onKeyDown={startEditingOnKey}
    >
      {rendered}
    </div>
  )
}

function placeCaretAtEnd(event: FocusEvent<HTMLTextAreaElement>) {
  const end = event.currentTarget.value.length
  event.currentTarget.setSelectionRange(end, end)
}
