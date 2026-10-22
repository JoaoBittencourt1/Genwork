import { Link } from '@tanstack/react-router'
import { BotIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/shared/ui/button'
import { Card, CardAction, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'

export function AssistantHomeCard() {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BotIcon className="size-4 text-primary" aria-hidden />
          {t('assistants.homeCard.title')}
        </CardTitle>
        <CardDescription>{t('assistants.homeCard.description')}</CardDescription>
        <CardAction>
          <Button variant="outline" size="sm" asChild>
            <Link to="/assistants">{t('assistants.homeCard.action')}</Link>
          </Button>
        </CardAction>
      </CardHeader>
    </Card>
  )
}
