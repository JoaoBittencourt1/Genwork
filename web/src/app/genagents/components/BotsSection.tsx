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
import { Textarea } from '@/shared/ui/textarea'
import { createBot, deleteBot, listAccounts, listBots, listMcpServers, listSkills, rotateBotSecret, updateBot } from '../datasource/genagentsDatasource'
import { botFormProblems, DEFAULT_MAX_TURNS, DEFAULT_TIMEOUT_MINUTES } from '../domain/botParameters'
import { buildBotPatch, draftFromBot, isEmptyPatch } from '../domain/botPatch'
import { keepModelOrFirst, modelsOfAccount } from '../domain/providers'
import { BOT_KINDS, REASONING_EFFORTS, type Bot, type BotKind, type ReasoningEffort } from '../domain/types'
import { genagentsQueryKeys } from './queryKeys'

type RoleOption = { token: string; name: string }

export function BotsSection({ canManage, roles }: { canManage: boolean; roles: RoleOption[] }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const bots = useQuery({ queryKey: genagentsQueryKeys.bots, queryFn: () => listBots() })
  const [editing, setEditing] = useState<Bot | 'new' | null>(null)
  const [removing, setRemoving] = useState<Bot | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.bots })
  const remove = useMutation({ mutationFn: (bot: Bot) => deleteBot(bot.token), onSuccess: refresh })
  const rotate = useMutation({ mutationFn: (bot: Bot) => rotateBotSecret(bot.token) })
  const toggle = useMutation({ mutationFn: (bot: Bot) => updateBot(bot.token, { enabled: !bot.enabled }), onSuccess: refresh })

  if (bots.isPending) return <Spinner />
  if (bots.isLoadingError) return <ErrorAlert error={bots.error} />

  return (
    <div className="flex flex-col gap-3">
      <ErrorAlert error={remove.error ?? rotate.error ?? toggle.error} />
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('genagents.bots.columns.name')}</TableHead>
              <TableHead>{t('genagents.bots.columns.kind')}</TableHead>
              <TableHead>{t('genagents.bots.columns.model')}</TableHead>
              <TableHead>{t('genagents.bots.columns.account')}</TableHead>
              <TableHead>{t('genagents.bots.columns.enabled')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t('genagents.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {bots.data.map((bot) => (
              <TableRow key={bot.token}>
                <TableCell className="font-medium">{bot.name}</TableCell>
                <TableCell>
                  <Badge variant="outline">{t(`genagents.bots.kinds.${bot.kind}`)}</Badge>
                </TableCell>
                <TableCell>{bot.model}</TableCell>
                <TableCell>{bot.account.name}</TableCell>
                <TableCell>
                  <Switch checked={bot.enabled} disabled={!canManage || toggle.isPending} aria-label={t('genagents.bots.toggle', { name: bot.name })} onCheckedChange={() => toggle.mutate(bot)} />
                </TableCell>
                <TableCell className="text-right">
                  {canManage ? (
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="ghost" onClick={() => setEditing(bot)}>
                        {t('genagents.bots.edit')}
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => rotate.mutate(bot)}>
                        {t('genagents.bots.rotateSecret')}
                      </Button>
                      <Button size="sm" variant="ghost" aria-label={t('genagents.bots.remove', { name: bot.name })} onClick={() => setRemoving(bot)}>
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
            {t('genagents.bots.add')}
          </Button>
        </div>
      ) : null}
      <Dialog open={editing !== null} onOpenChange={(open) => !open && setEditing(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          {editing !== null ? <BotForm bot={editing === 'new' ? null : editing} roles={roles} onDone={() => setEditing(null)} onSaved={refresh} /> : null}
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={removing !== null}
        title={t('genagents.bots.removeTitle')}
        description={t('genagents.bots.removeDescription', { name: removing?.name ?? '' })}
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

function BotForm({ bot, roles, onDone, onSaved }: { bot: Bot | null; roles: RoleOption[]; onDone: () => void; onSaved: () => void }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const accounts = useQuery({ queryKey: genagentsQueryKeys.accounts, queryFn: listAccounts })
  const skills = useQuery({ queryKey: genagentsQueryKeys.skills, queryFn: listSkills })
  const servers = useQuery({ queryKey: genagentsQueryKeys.mcpServers, queryFn: listMcpServers })
  const initial = bot ? draftFromBot(bot) : null
  const [name, setName] = useState(initial?.name ?? '')
  const [kind, setKind] = useState<BotKind>(initial?.kind ?? 'dev')
  const [accountToken, setAccountToken] = useState(initial?.accountToken ?? '')
  const [model, setModel] = useState(initial?.model ?? '')
  const [role, setRole] = useState(roles.find((candidate) => candidate.name === 'member')?.token ?? roles[0]?.token ?? '')
  const [instructions, setInstructions] = useState(initial?.instructions ?? '')
  const [effort, setEffort] = useState<ReasoningEffort>(initial?.effort ?? 'default')
  const [maxTurns, setMaxTurns] = useState(initial?.maxTurns ?? DEFAULT_MAX_TURNS)
  const [timeoutMinutes, setTimeoutMinutes] = useState(initial?.timeoutMinutes ?? DEFAULT_TIMEOUT_MINUTES)
  const [skillTokens, setSkillTokens] = useState<string[]>(initial?.skillTokens ?? [])
  const [mcpServerTokens, setMcpServerTokens] = useState<string[]>(initial?.mcpServerTokens ?? [])
  const [submitted, setSubmitted] = useState(false)
  const save = useMutation({
    mutationFn: async () => {
      if (!bot) {
        return createBot({ name: name.trim(), kind, accountToken, model, role, instructions: instructions.trim() === '' ? undefined : instructions, effort, maxTurns, timeoutMinutes, skillTokens, mcpServerTokens })
      }
      const patch = buildBotPatch(bot, { ...draftFromBot(bot), name, kind, accountToken, model, instructions, effort, maxTurns, timeoutMinutes, skillTokens, mcpServerTokens })
      return isEmptyPatch(patch) ? bot : updateBot(bot.token, patch)
    },
    onSuccess: () => {
      onSaved()
      onDone()
    },
  })
  const accountList = accounts.data ?? []
  const problems = botFormProblems({ name, accountToken, model, role: bot ? 'kept' : role, maxTurns, timeoutMinutes })

  function submit(event: FormEvent) {
    event.preventDefault()
    setSubmitted(true)
    if (Object.keys(problems).length === 0) save.mutate()
  }

  function chooseAccount(next: string) {
    setAccountToken(next)
    setModel(keepModelOrFirst(accountList, next, model))
  }

  function toggleToken(tokens: string[], token: string): string[] {
    return tokens.includes(token) ? tokens.filter((existing) => existing !== token) : [...tokens, token]
  }

  const problemOf = (field: keyof typeof problems) => (submitted && problems[field] ? t(`genagents.bots.problems.${problems[field]}`) : describeFieldError(save.error, field))

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{bot ? t('genagents.bots.editTitle', { name: bot.name }) : t('genagents.bots.add')}</DialogTitle>
      </DialogHeader>
      <ErrorAlert error={save.error} />
      <Field data-invalid={Boolean(problemOf('name'))}>
        <FieldLabel htmlFor="bot-name">{t('genagents.bots.columns.name')}</FieldLabel>
        <Input id="bot-name" value={name} autoFocus onChange={(event) => setName(event.target.value)} />
        <FieldError>{problemOf('name')}</FieldError>
      </Field>
      <Field>
        <FieldLabel htmlFor="bot-kind">{t('genagents.bots.columns.kind')}</FieldLabel>
        <Select value={kind} onValueChange={(value) => setKind(value as BotKind)}>
          <SelectTrigger id="bot-kind">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {BOT_KINDS.map((candidate) => (
              <SelectItem key={candidate} value={candidate}>
                {t(`genagents.bots.kinds.${candidate}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Field data-invalid={Boolean(problemOf('accountToken'))}>
        <FieldLabel htmlFor="bot-account">{t('genagents.bots.columns.account')}</FieldLabel>
        <Select value={accountToken} onValueChange={chooseAccount}>
          <SelectTrigger id="bot-account">
            <SelectValue placeholder={t('genagents.bots.chooseAccount')} />
          </SelectTrigger>
          <SelectContent>
            {accountList.map((account) => (
              <SelectItem key={account.token} value={account.token}>
                {account.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldError>{problemOf('accountToken')}</FieldError>
      </Field>
      <Field data-invalid={Boolean(problemOf('model'))}>
        <FieldLabel htmlFor="bot-model">{t('genagents.bots.columns.model')}</FieldLabel>
        <Select value={model} onValueChange={setModel}>
          <SelectTrigger id="bot-model">
            <SelectValue placeholder={t('genagents.bots.chooseModel')} />
          </SelectTrigger>
          <SelectContent>
            {modelsOfAccount(accountList, accountToken).map((candidate) => (
              <SelectItem key={candidate.id} value={candidate.id}>
                {candidate.displayName}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <FieldError>{problemOf('model')}</FieldError>
      </Field>
      {bot ? null : (
        <Field data-invalid={Boolean(problemOf('role'))}>
          <FieldLabel htmlFor="bot-role">{t('genagents.bots.role')}</FieldLabel>
          <Select value={role} onValueChange={setRole}>
            <SelectTrigger id="bot-role">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {roles.map((candidate) => (
                <SelectItem key={candidate.token} value={candidate.token}>
                  {candidate.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FieldError>{problemOf('role')}</FieldError>
        </Field>
      )}
      <Field>
        <FieldLabel htmlFor="bot-instructions">{t('genagents.bots.instructions')}</FieldLabel>
        <Textarea id="bot-instructions" rows={4} value={instructions} onChange={(event) => setInstructions(event.target.value)} />
      </Field>
      <div className="grid grid-cols-3 gap-3">
        <Field>
          <FieldLabel htmlFor="bot-effort">{t('genagents.bots.effort')}</FieldLabel>
          <Select value={effort} onValueChange={(value) => setEffort(value as ReasoningEffort)}>
            <SelectTrigger id="bot-effort">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {REASONING_EFFORTS.map((candidate) => (
                <SelectItem key={candidate} value={candidate}>
                  {t(`genagents.bots.efforts.${candidate}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field data-invalid={Boolean(problemOf('maxTurns'))}>
          <FieldLabel htmlFor="bot-turns">{t('genagents.bots.maxTurns')}</FieldLabel>
          <Input id="bot-turns" type="number" value={maxTurns} onChange={(event) => setMaxTurns(Number(event.target.value))} />
          <FieldError>{problemOf('maxTurns')}</FieldError>
        </Field>
        <Field data-invalid={Boolean(problemOf('timeoutMinutes'))}>
          <FieldLabel htmlFor="bot-timeout">{t('genagents.bots.timeoutMinutes')}</FieldLabel>
          <Input id="bot-timeout" type="number" value={timeoutMinutes} onChange={(event) => setTimeoutMinutes(Number(event.target.value))} />
          <FieldError>{problemOf('timeoutMinutes')}</FieldError>
        </Field>
      </div>
      <TokenChoice legend={t('genagents.bots.skills')} options={skills.data ?? []} selected={skillTokens} onToggle={(token) => setSkillTokens(toggleToken(skillTokens, token))} />
      <TokenChoice legend={t('genagents.bots.mcpServers')} options={servers.data ?? []} selected={mcpServerTokens} onToggle={(token) => setMcpServerTokens(toggleToken(mcpServerTokens, token))} />
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

function TokenChoice({ legend, options, selected, onToggle }: { legend: string; options: { token: string; name: string }[]; selected: string[]; onToggle: (token: string) => void }) {
  return (
    <fieldset className="flex flex-col gap-1">
      <legend className="text-sm font-medium">{legend}</legend>
      {options.map((option) => (
        <label key={option.token} className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={selected.includes(option.token)} onChange={() => onToggle(option.token)} />
          {option.name}
        </label>
      ))}
    </fieldset>
  )
}
