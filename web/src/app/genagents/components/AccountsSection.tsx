import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PlusIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { CopyableValue } from '@/shared/components/CopyableValue'
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
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import {
  addCustomModel,
  createAccount,
  listAccounts,
  listProviders,
  listRunners,
  removeAccount,
  removeCustomModel,
  updateAccount,
} from '../datasource/genagentsDatasource'
import { accountSignInHint, firstSupportedMode, supportsMode } from '../domain/providers'
import { runnersWithTool } from '../domain/runners'
import { ACCOUNT_MODES, PROVIDERS, type Account, type AccountMode, type NewAccount, type Provider } from '../domain/types'
import { genagentsQueryKeys } from './queryKeys'

export function AccountsSection({ canManage }: { canManage: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const accounts = useQuery({ queryKey: genagentsQueryKeys.accounts, queryFn: listAccounts, enabled: canManage })
  const [creating, setCreating] = useState(false)
  const [removing, setRemoving] = useState<Account | null>(null)
  const [editingModels, setEditingModels] = useState<Account | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.accounts })
  const remove = useMutation({ mutationFn: (account: Account) => removeAccount(account.token), onSuccess: refresh })
  const rename = useMutation({ mutationFn: ({ account, name }: { account: Account; name: string }) => updateAccount(account.token, { name }), onSuccess: refresh })

  if (accounts.isPending) return <Spinner />
  if (accounts.isLoadingError) return <ErrorAlert error={accounts.error} />

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">{t('genagents.accounts.termsNote')}</p>
      <ErrorAlert error={remove.error ?? rename.error} />
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('genagents.accounts.columns.name')}</TableHead>
              <TableHead>{t('genagents.accounts.columns.provider')}</TableHead>
              <TableHead>{t('genagents.accounts.columns.mode')}</TableHead>
              <TableHead>{t('genagents.accounts.columns.status')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t('genagents.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {accounts.data.map((account) => (
              <TableRow key={account.token}>
                <TableCell className="font-medium">{account.name}</TableCell>
                <TableCell>{t(`genagents.providers.${account.provider}`)}</TableCell>
                <TableCell>
                  {t(`genagents.accounts.modes.${account.mode}`)}
                  {account.apiKeyLast4 ? <span className="text-muted-foreground"> · …{account.apiKeyLast4}</span> : null}
                  {account.runner ? <span className="text-muted-foreground"> · {account.runner.name}</span> : null}
                </TableCell>
                <TableCell>
                  <AccountStatusBadge account={account} />
                  {accountSignInHint(account) ? <CopyableValue value={accountSignInHint(account) ?? ''} label={t('genagents.accounts.signInCommand')} /> : null}
                </TableCell>
                <TableCell className="text-right">
                  {canManage ? (
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="ghost" onClick={() => setEditingModels(account)}>
                        {t('genagents.accounts.models')}
                      </Button>
                      <Button size="sm" variant="ghost" aria-label={t('genagents.accounts.remove', { name: account.name })} onClick={() => setRemoving(account)}>
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
          <Button onClick={() => setCreating(true)}>
            <PlusIcon />
            {t('genagents.accounts.add')}
          </Button>
        </div>
      ) : null}
      <AccountDialog open={creating} onOpenChange={setCreating} onSaved={refresh} />
      <CustomModelsDialog account={editingModels} onClose={() => setEditingModels(null)} onChanged={refresh} />
      <ConfirmDialog
        open={removing !== null}
        title={t('genagents.accounts.removeTitle')}
        description={t('genagents.accounts.removeDescription', { name: removing?.name ?? '' })}
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

function AccountStatusBadge({ account }: { account: Account }) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-1">
      <Badge variant={account.status === 'ready' ? 'secondary' : account.status === 'error' ? 'destructive' : 'outline'}>{t(`genagents.accounts.statuses.${account.status}`)}</Badge>
      {account.lastError ? <span className="text-xs text-destructive">{account.lastError}</span> : null}
    </div>
  )
}

function AccountDialog({ open, onOpenChange, onSaved }: { open: boolean; onOpenChange: (open: boolean) => void; onSaved: () => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>{open ? <AccountForm onDone={() => onOpenChange(false)} onSaved={onSaved} /> : null}</DialogContent>
    </Dialog>
  )
}

function AccountForm({ onDone, onSaved }: { onDone: () => void; onSaved: () => void }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const providers = useQuery({ queryKey: genagentsQueryKeys.providers, queryFn: listProviders })
  const runners = useQuery({ queryKey: genagentsQueryKeys.runners, queryFn: listRunners })
  const [name, setName] = useState('')
  const [provider, setProvider] = useState<Provider>('anthropic')
  const [mode, setMode] = useState<AccountMode>('api_key')
  const [apiKey, setApiKey] = useState('')
  const [runnerToken, setRunnerToken] = useState('')
  const save = useMutation({
    mutationFn: (account: NewAccount) => createAccount(account),
    onSuccess: () => {
      onSaved()
      onDone()
    },
  })
  const catalog = providers.data ?? []
  const cli = catalog.find((entry) => entry.provider === provider)?.cli ?? ''
  const eligibleRunners = runnersWithTool(runners.data ?? [], cli)

  function chooseProvider(next: Provider) {
    setProvider(next)
    setMode(firstSupportedMode(catalog, next, mode))
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate({ name, provider, mode, ...(mode === 'api_key' ? { apiKey } : { runnerToken }) })
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{t('genagents.accounts.add')}</DialogTitle>
      </DialogHeader>
      <ErrorAlert error={save.error} />
      <Field data-invalid={Boolean(describeFieldError(save.error, 'name'))}>
        <FieldLabel htmlFor="account-name">{t('genagents.accounts.columns.name')}</FieldLabel>
        <Input id="account-name" value={name} required autoFocus onChange={(event) => setName(event.target.value)} />
        <FieldError>{describeFieldError(save.error, 'name')}</FieldError>
      </Field>
      <Field>
        <FieldLabel htmlFor="account-provider">{t('genagents.accounts.columns.provider')}</FieldLabel>
        <Select value={provider} onValueChange={(value) => chooseProvider(value as Provider)}>
          <SelectTrigger id="account-provider">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PROVIDERS.map((candidate) => (
              <SelectItem key={candidate} value={candidate}>
                {t(`genagents.providers.${candidate}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Field>
        <FieldLabel htmlFor="account-mode">{t('genagents.accounts.columns.mode')}</FieldLabel>
        <Select value={mode} onValueChange={(value) => setMode(value as AccountMode)}>
          <SelectTrigger id="account-mode">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {ACCOUNT_MODES.filter((candidate) => supportsMode(catalog, provider, candidate)).map((candidate) => (
              <SelectItem key={candidate} value={candidate}>
                {t(`genagents.accounts.modes.${candidate}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      {mode === 'api_key' ? (
        <Field data-invalid={Boolean(describeFieldError(save.error, 'apiKey'))}>
          <FieldLabel htmlFor="account-key">{t('genagents.accounts.apiKey')}</FieldLabel>
          <Input id="account-key" type="password" autoComplete="off" value={apiKey} required onChange={(event) => setApiKey(event.target.value)} />
          <FieldError>{describeFieldError(save.error, 'apiKey')}</FieldError>
        </Field>
      ) : (
        <Field data-invalid={Boolean(describeFieldError(save.error, 'runnerToken'))}>
          <FieldLabel htmlFor="account-runner">{t('genagents.accounts.runner')}</FieldLabel>
          <Select value={runnerToken} onValueChange={setRunnerToken}>
            <SelectTrigger id="account-runner">
              <SelectValue placeholder={t('genagents.accounts.chooseRunner')} />
            </SelectTrigger>
            <SelectContent>
              {eligibleRunners.map((runner) => (
                <SelectItem key={runner.token} value={runner.token}>
                  {runner.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FieldError>{describeFieldError(save.error, 'runnerToken')}</FieldError>
        </Field>
      )}
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

function CustomModelsDialog({ account, onClose, onChanged }: { account: Account | null; onClose: () => void; onChanged: () => void }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [modelId, setModelId] = useState('')
  const accounts = useQuery({ queryKey: genagentsQueryKeys.accounts, queryFn: listAccounts, enabled: account !== null })
  const current = accounts.data?.find((candidate) => candidate.token === account?.token) ?? account
  const afterChange = () => {
    onChanged()
    void queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.accounts })
  }
  const add = useMutation({
    mutationFn: () => addCustomModel(account?.token ?? '', modelId.trim()),
    onSuccess: () => {
      setModelId('')
      afterChange()
    },
  })
  const remove = useMutation({ mutationFn: (modelToken: string) => removeCustomModel(account?.token ?? '', modelToken), onSuccess: afterChange })

  return (
    <Dialog open={account !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('genagents.accounts.modelsOf', { name: current?.name ?? '' })}</DialogTitle>
        </DialogHeader>
        <ErrorAlert error={add.error ?? remove.error} />
        <ul className="flex flex-col gap-1 text-sm">
          {(current?.models ?? []).map((model) => (
            <li key={model.id} className="flex items-center justify-between gap-2">
              <span>
                {model.displayName} <span className="text-muted-foreground">({model.id})</span>
              </span>
              {model.custom && model.token ? (
                <Button size="sm" variant="ghost" aria-label={t('genagents.accounts.removeModel', { id: model.id })} onClick={() => remove.mutate(model.token ?? '')}>
                  <Trash2Icon />
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
        <form
          className="flex gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            if (modelId.trim() !== '') add.mutate()
          }}
        >
          <Input value={modelId} aria-label={t('genagents.accounts.customModel')} placeholder={t('genagents.accounts.customModelPlaceholder')} onChange={(event) => setModelId(event.target.value)} />
          <Button type="submit" disabled={add.isPending || modelId.trim() === ''}>
            {t('genagents.accounts.addModel')}
          </Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
