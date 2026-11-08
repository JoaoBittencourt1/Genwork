import { useMutation } from '@tanstack/react-query'
import { KeyRoundIcon, UnlinkIcon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { ApiError } from '@/shared/datasource/httpClient'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Field, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { disconnectMcpOAuth, startMcpOAuth } from '../datasource/genagentsDatasource'
import { canDisconnectMcpOAuth, mcpOAuthState, needsManualClient } from '../domain/mcpOAuth'
import type { McpServer, StartMcpOAuth } from '../domain/types'

// McpOAuthControls connects an HTTP MCP server through OAuth: the browser goes to the server's authorization page and
// comes back to the MCP servers page.
export function McpOAuthControls({ server, onChanged }: { server: McpServer; onChanged: () => void }) {
  const { t } = useTranslation()
  const [needsClient, setNeedsClient] = useState(false)
  const connect = useMutation({
    mutationFn: (input: StartMcpOAuth) => startMcpOAuth(server.token, input),
    onSuccess: (authorizationUrl) => window.location.assign(authorizationUrl),
    onError: (error) => setNeedsClient(needsManualClient(error instanceof ApiError ? error.code : null)),
  })
  const disconnect = useMutation({ mutationFn: () => disconnectMcpOAuth(server.token), onSuccess: onChanged })
  const state = mcpOAuthState(server)
  if (state === 'unavailable') return null

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center justify-end gap-1">
        <Badge variant={state === 'connected' ? 'secondary' : state === 'error' ? 'destructive' : 'outline'}>{t(`genagents.mcpServers.oauth.states.${state}`)}</Badge>
        <Button size="sm" variant="ghost" onClick={() => connect.mutate({})} disabled={connect.isPending}>
          <KeyRoundIcon />
          {t(state === 'connected' ? 'genagents.mcpServers.oauth.reconnect' : 'genagents.mcpServers.oauth.connect')}
        </Button>
        {canDisconnectMcpOAuth(server) ? (
          <Button size="sm" variant="ghost" onClick={() => disconnect.mutate()} disabled={disconnect.isPending}>
            <UnlinkIcon />
            {t('genagents.mcpServers.oauth.disconnect')}
          </Button>
        ) : null}
      </div>
      {needsClient ? <ManualClientForm onSubmit={(input) => connect.mutate(input)} pending={connect.isPending} /> : <ErrorAlert error={connect.error ?? disconnect.error} />}
    </div>
  )
}

function ManualClientForm({ onSubmit, pending }: { onSubmit: (input: StartMcpOAuth) => void; pending: boolean }) {
  const { t } = useTranslation()
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')

  function submit(event: FormEvent) {
    event.preventDefault()
    onSubmit({ clientId: clientId.trim(), ...(clientSecret ? { clientSecret } : {}) })
  }

  return (
    <form className="flex w-full max-w-sm flex-col gap-2 rounded-md border p-3 text-left" onSubmit={submit}>
      <p className="text-xs text-muted-foreground">{t('genagents.mcpServers.oauth.clientRequired')}</p>
      <Field>
        <FieldLabel htmlFor="oauth-client-id">{t('genagents.mcpServers.oauth.clientId')}</FieldLabel>
        <Input id="oauth-client-id" value={clientId} required onChange={(event) => setClientId(event.target.value)} />
      </Field>
      <Field>
        <FieldLabel htmlFor="oauth-client-secret">{t('genagents.mcpServers.oauth.clientSecret')}</FieldLabel>
        <Input id="oauth-client-secret" type="password" autoComplete="off" value={clientSecret} onChange={(event) => setClientSecret(event.target.value)} />
      </Field>
      <div>
        <Button type="submit" size="sm" disabled={pending || clientId.trim() === ''}>
          {t('genagents.mcpServers.oauth.connect')}
        </Button>
      </div>
    </form>
  )
}
