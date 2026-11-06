import { SendIcon, SquareIcon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { isValidMessage, MAX_MESSAGE_LENGTH } from '@/shared/domain/conversation'
import { Button } from '@/shared/ui/button'
import { Textarea } from '@/shared/ui/textarea'

type ConversationMessageBoxProps = {
  disabled: boolean
  isSending: boolean
  // onStop is set only while a run can be stopped.
  onStop?: () => void
  onSend: (text: string) => void
}

export function ConversationMessageBox({ disabled, isSending, onStop, onSend }: ConversationMessageBoxProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!isValidMessage(text)) return
    onSend(text.trim())
    setText('')
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-2">
      <Textarea
        value={text}
        rows={2}
        maxLength={MAX_MESSAGE_LENGTH}
        disabled={disabled}
        aria-label={t('conversation.messageLabel')}
        placeholder={t('conversation.messagePlaceholder')}
        onChange={(event) => setText(event.target.value)}
      />
      <div className="flex justify-end gap-2">
        {onStop ? (
          <Button type="button" variant="outline" onClick={onStop}>
            <SquareIcon />
            {t('conversation.stop')}
          </Button>
        ) : null}
        <Button type="submit" disabled={disabled || isSending || !isValidMessage(text)}>
          <SendIcon />
          {t('conversation.send')}
        </Button>
      </div>
    </form>
  )
}
