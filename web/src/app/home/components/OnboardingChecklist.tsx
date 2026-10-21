import { Link } from '@tanstack/react-router'
import { CheckCircle2Icon, CircleIcon, PartyPopperIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Progress } from '@/shared/ui/progress'
import type { OnboardingChecklist as OnboardingChecklistModel } from '../domain/onboarding'

export function OnboardingChecklist({ checklist }: { checklist: OnboardingChecklistModel }) {
  const { t } = useTranslation()
  const { doneCount, totalCount } = checklist
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('home.onboarding.title')}</CardTitle>
        <CardDescription>{t('home.onboarding.description')}</CardDescription>
        <CardAction className="text-sm text-muted-foreground tabular-nums">
          {t('home.onboarding.progress', { done: doneCount, total: totalCount })}
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        {checklist.isComplete ? (
          <p className="flex items-center gap-2 text-sm">
            <PartyPopperIcon className="size-4 text-primary" aria-hidden />
            {t('home.onboarding.complete')}
          </p>
        ) : (
          <Progress value={(doneCount / totalCount) * 100} aria-label={t('home.onboarding.progress', { done: doneCount, total: totalCount })} />
        )}
        <ol className="space-y-3">
          {checklist.steps.map((step) => (
            <li key={step.id} className="flex items-start gap-3">
              {step.done ? (
                <CheckCircle2Icon className="mt-0.5 size-5 shrink-0 text-primary" aria-label={t('home.onboarding.stepDone')} />
              ) : (
                <CircleIcon className="mt-0.5 size-5 shrink-0 text-muted-foreground" aria-label={t('home.onboarding.stepOpen')} />
              )}
              <div className="min-w-0 flex-1">
                <p className={cn('text-sm font-medium', step.done && 'text-muted-foreground line-through')}>
                  {t(`home.onboarding.steps.${step.id}.title`)}
                </p>
                <p className="text-sm text-muted-foreground">{t(`home.onboarding.steps.${step.id}.description`)}</p>
              </div>
              {step.done ? null : (
                <Button size="sm" variant="outline" asChild>
                  <Link to={step.linkTo}>{t('home.onboarding.start')}</Link>
                </Button>
              )}
            </li>
          ))}
        </ol>
      </CardContent>
    </Card>
  )
}
