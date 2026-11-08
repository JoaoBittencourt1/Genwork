import { useMutation } from '@tanstack/react-query'
import { ExternalLinkIcon, LogInIcon, XIcon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { CopyableValue } from '@/shared/components/CopyableValue'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { cancelAccountSignIn, startAccountSignIn, submitAccountSignInCode } from '../datasource/genagentsDatasource'
import { accountSignInHint } from '../domain/providers'
import { canStartSignIn, canSubmitSignInCode, signInButtonLabelKey, signInErrorKey, signInStep } from '../domain/signIns'
import type { Account, AccountSignIn } from '../domain/types'

// AccountSignInPanel drives the web sign-in of a subscription account: the provider CLI runs on the account's runner,
// and this panel shows its URL and takes the code the provider's page displays.
export function AccountSignInPanel({ account, onChanged }: { account: Account; onChanged: () => void }) {
  const { t } = useTranslation()
  const start = useMutation({ mutationFn: () => startAccountSignIn(account.token), onSuccess: onChanged })
  const cancel = useMutation({ mutationFn: (signIn: AccountSignIn) => cancelAccountSignIn(account.token, signIn.token), onSuccess: onChanged })
  const step = signInStep(account)
  const fallback = accountSignInHint(account)
  const signIn = account.signIn

  return (
    <div className="mt-2 flex flex-col gap-2">
      <ErrorAlert error={start.error ?? cancel.error} />
      {step === 'ended' && signIn ? <SignInEnded signIn={signIn} /> : null}
      {canStartSignIn(account) ? (
        <div>
          <Button size="sm" onClick={() => start.mutate()} disabled={start.isPending}>
            <LogInIcon />
            {t(signInButtonLabelKey(account), { provider: t(`genagents.providers.${account.provider}`) })}
          </Button>
        </div>
      ) : null}
      {step === 'waiting_for_runner' ? (
        <p className="flex items-center gap-2 text-xs text-muted-foreground">
          <Spinner />
          {t('genagents.accounts.signIn.waitingForRunner', { runner: account.runner?.name ?? '' })}
        </p>
      ) : null}
      {step === 'awaiting_user' && signIn ? <SignInPrompt account={account} signIn={signIn} onChanged={onChanged} /> : null}
      {step === 'verifying' ? (
        <p className="flex items-center gap-2 text-xs text-muted-foreground">
          <Spinner />
          {t('genagents.accounts.signIn.verifying')}
        </p>
      ) : null}
      {signIn && (step === 'waiting_for_runner' || step === 'awaiting_user') ? (
        <div>
          <Button size="sm" variant="ghost" onClick={() => cancel.mutate(signIn)} disabled={cancel.isPending}>
            <XIcon />
            {t('genagents.accounts.signIn.cancel')}
          </Button>
        </div>
      ) : null}
      {fallback ? (
        <details className="text-xs text-muted-foreground">
          <summary className="cursor-pointer">{t('genagents.accounts.signIn.fallback')}</summary>
          <div className="mt-2">
            <CopyableValue value={fallback} label={t('genagents.accounts.signInCommand')} />
          </div>
        </details>
      ) : null}
    </div>
  )
}

function SignInPrompt({ account, signIn, onChanged }: { account: Account; signIn: AccountSignIn; onChanged: () => void }) {
  const { t } = useTranslation()
  const [code, setCode] = useState('')
  const submit = useMutation({
    mutationFn: () => submitAccountSignInCode(account.token, signIn.token, code.trim()),
    onSuccess: () => {
      setCode('')
      onChanged()
    },
  })

  function submitCode(event: FormEvent) {
    event.preventDefault()
    if (canSubmitSignInCode(code)) submit.mutate()
  }

  return (
    <div className="flex flex-col gap-2 rounded-md border p-3">
      <p className="text-xs text-muted-foreground">{t(signIn.expectsCode ? 'genagents.accounts.signIn.openAndPaste' : 'genagents.accounts.signIn.openAndAuthorize')}</p>
      {signIn.url ? (
        <div>
          <Button size="sm" variant="outline" asChild>
            <a href={signIn.url} target="_blank" rel="noopener noreferrer">
              <ExternalLinkIcon />
              {t('genagents.accounts.signIn.open', { host: signIn.urlHost ?? '' })}
            </a>
          </Button>
        </div>
      ) : null}
      {signIn.userCode ? <CopyableValue value={signIn.userCode} label={t('genagents.accounts.signIn.userCode')} /> : null}
      {signIn.expectsCode ? (
        <form className="flex gap-2" onSubmit={submitCode}>
          <Input
            value={code}
            autoComplete="off"
            aria-label={t('genagents.accounts.signIn.code')}
            placeholder={t('genagents.accounts.signIn.codePlaceholder')}
            onChange={(event) => setCode(event.target.value)}
          />
          <Button type="submit" size="sm" disabled={submit.isPending || !canSubmitSignInCode(code)}>
            {t('genagents.accounts.signIn.submitCode')}
          </Button>
        </form>
      ) : null}
      <ErrorAlert error={submit.error} />
    </div>
  )
}

function SignInEnded({ signIn }: { signIn: AccountSignIn }) {
  const { t } = useTranslation()
  const failureKey = signInErrorKey(signIn.failure)
  return (
    <p className="text-xs text-muted-foreground">
      {t(`genagents.accounts.signIn.states.${signIn.state}`)}
      {signIn.state === 'failed' && signIn.failure ? `: ${failureKey ? t(failureKey) : signIn.failure}` : null}
    </p>
  )
}
