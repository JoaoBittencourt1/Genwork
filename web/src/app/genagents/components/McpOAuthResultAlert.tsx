import { useLocation } from '@tanstack/react-router'
import { CircleAlertIcon, CircleCheckIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { mcpOAuthReasonKey, readMcpOAuthResult } from '../domain/mcpOAuth'

// McpOAuthResultAlert shows how the OAuth round trip ended when the callback lands back on the MCP servers page.
export function McpOAuthResultAlert() {
  const { t } = useTranslation()
  const result = readMcpOAuthResult(useLocation({ select: (location) => location.searchStr }))
  if (result === null) return null
  return (
    <Alert variant={result.connected ? 'default' : 'destructive'}>
      {result.connected ? <CircleCheckIcon /> : <CircleAlertIcon />}
      <AlertDescription>{result.connected ? t('genagents.mcpServers.oauth.connected') : t(mcpOAuthReasonKey(result.reason))}</AlertDescription>
    </Alert>
  )
}
