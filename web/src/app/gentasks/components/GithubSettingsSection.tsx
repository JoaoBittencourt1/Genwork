import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { InfoIcon, PlusIcon, XIcon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Alert, AlertDescription } from '@/shared/ui/alert'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { Field, FieldDescription, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import { Spinner } from '@/shared/ui/spinner'
import { loadError } from '@/shared/domain/errors'
import { readBoardGithubSettings, updateBoardGithubSettings } from '../datasource/githubDatasource'
import { resolveColumnTitle } from '../domain/columnName'
import {
  addFinalBranch,
  buildGithubSettingsPatch,
  draftFromGithubSettings,
  finalBranchesProblem,
  GITHUB_COLUMN_FIELDS,
  MAX_BRANCH_NAME_LENGTH,
  removeFinalBranch,
  type BoardGithubSettingsDraft,
  type GithubColumnField,
} from '../domain/githubSettings'
import { isPatchEmpty } from '../domain/patch'
import { taskIdentifier } from '../domain/taskPrefix'
import type { BoardGithubSettings, BoardView } from '../domain/types'
import { gentasksQueryKeys } from './queryKeys'
import { useGithubConnection } from './useGithubConnection'

// Select items cannot have an empty value; column tokens are 22 characters, so this never collides with one.
const DO_NOT_MOVE = 'do-not-move'

const FIELD_LABEL_KEYS: Record<GithubColumnField, string> = {
  openedColumnToken: 'gentasks.github.settings.opened',
  reviewRequestedColumnToken: 'gentasks.github.settings.reviewRequested',
  mergedColumnToken: 'gentasks.github.settings.merged',
}

type GithubSettingsSectionProps = {
  board: BoardView
  canManageGithub: boolean
}

export function GithubSettingsSection({ board, canManageGithub }: GithubSettingsSectionProps) {
  const { t } = useTranslation()
  const connection = useGithubConnection()
  const settings = useQuery({
    queryKey: gentasksQueryKeys.boardGithubSettings(board.token),
    queryFn: () => readBoardGithubSettings(board.token),
  })

  return (
    <div className="flex flex-col gap-5">
      {connection.data && connection.data.state !== 'connected' ? (
        <Alert>
          <InfoIcon />
          <AlertDescription>
            <p>
              {t('gentasks.github.settings.notConnected')}{' '}
              {canManageGithub ? (
                <Link to="/tasks/integrations/github" className="underline underline-offset-4">
                  {t('gentasks.github.settings.connectLink')}
                </Link>
              ) : null}
            </p>
          </AlertDescription>
        </Alert>
      ) : null}
      {settings.isPending ? <Spinner /> : null}
      <ErrorAlert error={loadError(settings)} />
      {settings.data ? <GithubSettingsForm key={JSON.stringify(settings.data)} board={board} settings={settings.data} /> : null}
    </div>
  )
}

function GithubSettingsForm({ board, settings }: { board: BoardView; settings: BoardGithubSettings }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<BoardGithubSettingsDraft>(draftFromGithubSettings(settings))
  const [newBranch, setNewBranch] = useState('')
  const patch = buildGithubSettingsPatch(settings, draft)
  const branchesProblem = finalBranchesProblem(draft.finalBranches)
  const save = useMutation({
    mutationFn: () => updateBoardGithubSettings(board.token, patch),
    onSuccess: (saved) => {
      queryClient.setQueryData(gentasksQueryKeys.boardGithubSettings(board.token), saved)
      toast.success(t('gentasks.settings.saved'))
    },
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  function addBranch() {
    setDraft({ ...draft, finalBranches: addFinalBranch(draft.finalBranches, newBranch) })
    setNewBranch('')
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-5">
      <ErrorAlert error={save.error} />
      {GITHUB_COLUMN_FIELDS.map((field) => (
        <Field key={field}>
          <FieldLabel htmlFor={`github-${field}`}>{t(FIELD_LABEL_KEYS[field])}</FieldLabel>
          <Select
            value={draft[field] ?? DO_NOT_MOVE}
            onValueChange={(value) => setDraft({ ...draft, [field]: value === DO_NOT_MOVE ? null : value })}
          >
            <SelectTrigger id={`github-${field}`} className="w-64">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={DO_NOT_MOVE}>{t('gentasks.github.settings.doNotMove')}</SelectItem>
              {board.columns.map((column) => (
                <SelectItem key={column.token} value={column.token}>
                  {resolveColumnTitle(column, t)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {field === 'openedColumnToken' ? (
            <FieldDescription>{t('gentasks.github.settings.keyHint', { example: taskIdentifier(board.taskPrefix, 123) })}</FieldDescription>
          ) : null}
        </Field>
      ))}
      <Field data-invalid={Boolean(branchesProblem)}>
        <FieldLabel htmlFor="github-new-branch">{t('gentasks.github.settings.finalBranches')}</FieldLabel>
        <ul className="flex flex-wrap gap-2" aria-label={t('gentasks.github.settings.finalBranches')}>
          {draft.finalBranches.map((branch) => (
            <li key={branch}>
              <Badge variant="secondary" className="gap-1 pr-1 font-mono">
                {branch}
                <button
                  type="button"
                  className="rounded-sm opacity-70 hover:opacity-100"
                  aria-label={t('gentasks.github.settings.removeBranch', { branch })}
                  onClick={() => setDraft({ ...draft, finalBranches: removeFinalBranch(draft.finalBranches, branch) })}
                >
                  <XIcon className="size-3" />
                </button>
              </Badge>
            </li>
          ))}
        </ul>
        <div className="flex gap-2">
          <Input
            id="github-new-branch"
            value={newBranch}
            maxLength={MAX_BRANCH_NAME_LENGTH}
            placeholder={t('gentasks.github.settings.branchPlaceholder')}
            className="w-64 font-mono"
            onChange={(event) => setNewBranch(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault()
                addBranch()
              }
            }}
          />
          <Button type="button" size="sm" variant="outline" onClick={addBranch}>
            <PlusIcon />
            {t('gentasks.github.settings.addBranch')}
          </Button>
        </div>
        <FieldDescription>{t('gentasks.github.settings.finalBranchesDescription')}</FieldDescription>
        {branchesProblem ? <FieldError>{t(`gentasks.github.settings.branchProblems.${branchesProblem}`)}</FieldError> : null}
      </Field>
      <div>
        <Button type="submit" size="sm" disabled={save.isPending || isPatchEmpty(patch) || branchesProblem !== null}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </Button>
      </div>
    </form>
  )
}
