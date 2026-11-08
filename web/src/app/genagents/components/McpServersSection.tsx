import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PlusIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Spinner } from '@/shared/ui/spinner'
import { Switch } from '@/shared/ui/switch'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { createMcpServer, deleteMcpServer, listMcpServers, updateMcpServer } from '../datasource/genagentsDatasource'
import { MCP_TRANSPORTS, type McpEntry, type McpEntryInput, type McpServer, type McpTransport } from '../domain/types'
import { McpOAuthControls } from './McpOAuthControls'
import { McpOAuthResultAlert } from './McpOAuthResultAlert'
import { genagentsQueryKeys } from './queryKeys'

export function McpServersSection({ canManage }: { canManage: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const servers = useQuery({ queryKey: genagentsQueryKeys.mcpServers, queryFn: listMcpServers, enabled: canManage })
  const [editing, setEditing] = useState<McpServer | 'new' | null>(null)
  const [removing, setRemoving] = useState<McpServer | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.mcpServers })
  const remove = useMutation({ mutationFn: (server: McpServer) => deleteMcpServer(server.token), onSuccess: refresh })

  if (servers.isPending) return <Spinner />
  if (servers.isLoadingError) return <ErrorAlert error={servers.error} />

  return (
    <div className="flex flex-col gap-3">
      <McpOAuthResultAlert />
      <ErrorAlert error={remove.error} />
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('genagents.mcpServers.columns.name')}</TableHead>
              <TableHead>{t('genagents.mcpServers.columns.transport')}</TableHead>
              <TableHead>{t('genagents.mcpServers.columns.target')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t('genagents.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {servers.data.map((server) => (
              <TableRow key={server.token}>
                <TableCell className="font-medium">{server.name}</TableCell>
                <TableCell>
                  <Badge variant="outline">{server.transport}</Badge>
                </TableCell>
                <TableCell className="max-w-md truncate font-mono text-xs">{server.url ?? server.command}</TableCell>
                <TableCell className="text-right">
                  {canManage ? (
                    <div className="flex flex-wrap items-start justify-end gap-1">
                      <McpOAuthControls server={server} onChanged={refresh} />
                      <Button size="sm" variant="ghost" onClick={() => setEditing(server)}>
                        {t('genagents.mcpServers.edit')}
                      </Button>
                      <Button size="sm" variant="ghost" aria-label={t('genagents.mcpServers.remove', { name: server.name })} onClick={() => setRemoving(server)}>
                        <Trash2Icon />
                      </Button>
                    </div>
                  ) : null}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {canManage ? (
        <div>
          <Button onClick={() => setEditing('new')}>
            <PlusIcon />
            {t('genagents.mcpServers.add')}
          </Button>
        </div>
      ) : null}
      <Dialog open={editing !== null} onOpenChange={(open) => !open && setEditing(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          {editing !== null ? <McpServerForm server={editing === 'new' ? null : editing} onDone={() => setEditing(null)} onSaved={refresh} /> : null}
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={removing !== null}
        title={t('genagents.mcpServers.removeTitle')}
        description={t('genagents.mcpServers.removeDescription', { name: removing?.name ?? '' })}
        confirmLabel={t('common.delete')}
        onOpenChange={(open) => !open && setRemoving(null)}
        onConfirm={() => {
          if (removing) remove.mutate(removing)
          setRemoving(null)
        }}
      />
    </div>
  )
}

type EntryDraft = { name: string; secret: boolean; value: string; keepsStoredValue: boolean }

function draftsOf(entries: McpEntry[]): EntryDraft[] {
  return entries.map((entry) => ({ name: entry.name, secret: entry.secret, value: entry.value ?? '', keepsStoredValue: entry.secret && entry.hasValue }))
}

// inputsOf leaves a secret's value out when the admin kept the stored one, so the API keeps it too.
function inputsOf(drafts: EntryDraft[]): McpEntryInput[] {
  return drafts
    .filter((draft) => draft.name.trim() !== '')
    .map((draft) => (draft.secret && draft.keepsStoredValue && draft.value === '' ? { name: draft.name.trim(), secret: true } : { name: draft.name.trim(), secret: draft.secret, value: draft.value }))
}

function McpServerForm({ server, onDone, onSaved }: { server: McpServer | null; onDone: () => void; onSaved: () => void }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const [name, setName] = useState(server?.name ?? '')
  const [transport, setTransport] = useState<McpTransport>(server?.transport ?? 'http')
  const [url, setUrl] = useState(server?.url ?? '')
  const [command, setCommand] = useState(server?.command ?? '')
  const [args, setArgs] = useState((server?.args ?? []).join(' '))
  const [entries, setEntries] = useState<EntryDraft[]>(draftsOf(server ? (server.transport === 'http' ? server.headers : server.env) : []))
  const save = useMutation({
    mutationFn: () => {
      const argList = args.trim() === '' ? [] : args.trim().split(/\s+/)
      if (server) {
        return updateMcpServer(server.token, transport === 'http' ? { name, url, headers: inputsOf(entries) } : { name, command, args: argList, env: inputsOf(entries) })
      }
      return createMcpServer(transport === 'http' ? { name, transport, url, headers: inputsOf(entries) } : { name, transport, command, args: argList, env: inputsOf(entries) })
    },
    onSuccess: () => {
      onSaved()
      onDone()
    },
  })
  const entriesField = transport === 'http' ? 'headers' : 'env'

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  function updateEntry(index: number, change: Partial<EntryDraft>) {
    setEntries(entries.map((entry, position) => (position === index ? { ...entry, ...change } : entry)))
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{server ? t('genagents.mcpServers.editTitle', { name: server.name }) : t('genagents.mcpServers.add')}</DialogTitle>
      </DialogHeader>
      <ErrorAlert error={save.error} />
      <Field data-invalid={Boolean(describeFieldError(save.error, 'name'))}>
        <FieldLabel htmlFor="mcp-name">{t('genagents.mcpServers.columns.name')}</FieldLabel>
        <Input id="mcp-name" value={name} onChange={(event) => setName(event.target.value)} />
        <FieldError>{describeFieldError(save.error, 'name')}</FieldError>
      </Field>
      <Field>
        <FieldLabel htmlFor="mcp-transport">{t('genagents.mcpServers.columns.transport')}</FieldLabel>
        <Select value={transport} onValueChange={(value) => setTransport(value as McpTransport)} disabled={server !== null}>
          <SelectTrigger id="mcp-transport">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {MCP_TRANSPORTS.map((candidate) => (
              <SelectItem key={candidate} value={candidate}>
                {candidate}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      {transport === 'http' ? (
        <Field data-invalid={Boolean(describeFieldError(save.error, 'url'))}>
          <FieldLabel htmlFor="mcp-url">{t('genagents.mcpServers.url')}</FieldLabel>
          <Input id="mcp-url" value={url} placeholder="https://mcp.example.com/mcp" onChange={(event) => setUrl(event.target.value)} />
          <FieldError>{describeFieldError(save.error, 'url')}</FieldError>
        </Field>
      ) : (
        <>
          <p className="text-xs text-muted-foreground">{t('genagents.mcpServers.stdioNote')}</p>
          <Field data-invalid={Boolean(describeFieldError(save.error, 'command'))}>
            <FieldLabel htmlFor="mcp-command">{t('genagents.mcpServers.command')}</FieldLabel>
            <Input id="mcp-command" value={command} onChange={(event) => setCommand(event.target.value)} />
            <FieldError>{describeFieldError(save.error, 'command')}</FieldError>
          </Field>
          <Field>
            <FieldLabel htmlFor="mcp-args">{t('genagents.mcpServers.args')}</FieldLabel>
            <Input id="mcp-args" value={args} onChange={(event) => setArgs(event.target.value)} />
          </Field>
        </>
      )}
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">{t(`genagents.mcpServers.${entriesField}`)}</legend>
        {entries.map((entry, index) => (
          <div key={index} className="flex items-center gap-2">
            <Input value={entry.name} aria-label={t('genagents.mcpServers.entryName')} onChange={(event) => updateEntry(index, { name: event.target.value })} />
            <Input
              type={entry.secret ? 'password' : 'text'}
              value={entry.value}
              autoComplete="off"
              aria-label={t('genagents.mcpServers.entryValue')}
              placeholder={entry.secret && entry.keepsStoredValue ? t('genagents.mcpServers.keepStored') : ''}
              onChange={(event) => updateEntry(index, { value: event.target.value })}
            />
            <label className="flex items-center gap-1 text-xs">
              <Switch checked={entry.secret} onCheckedChange={(checked) => updateEntry(index, { secret: checked })} />
              {t('genagents.mcpServers.secret')}
            </label>
            <Button type="button" size="sm" variant="ghost" aria-label={t('genagents.mcpServers.removeEntry')} onClick={() => setEntries(entries.filter((_, position) => position !== index))}>
              <Trash2Icon />
            </Button>
          </div>
        ))}
        <FieldError>{describeFieldError(save.error, entriesField)}</FieldError>
        <div>
          <Button type="button" variant="outline" size="sm" onClick={() => setEntries([...entries, { name: '', secret: false, value: '', keepsStoredValue: false }])}>
            {t('genagents.mcpServers.addEntry')}
          </Button>
        </div>
      </fieldset>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </Button>
      </DialogFooter>
    </form>
  )
}
