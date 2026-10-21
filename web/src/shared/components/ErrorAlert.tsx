import { CircleAlertIcon } from 'lucide-react'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Alert, AlertDescription } from '@/shared/ui/alert'

export function ErrorAlert({ error }: { error: unknown }) {
  const { describeError } = useErrorMessage()
  if (!error) return null
  return (
    <Alert variant="destructive">
      <CircleAlertIcon />
      <AlertDescription>{describeError(error)}</AlertDescription>
    </Alert>
  )
}
