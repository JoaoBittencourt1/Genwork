import { TerminalIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { GENWORK_COMMANDS, genworkCommandLine } from '../domain/cliCommands'

export function CliCommandGuide() {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <TerminalIcon className="size-4 text-primary" aria-hidden />
          {t('home.cli.title')}
        </CardTitle>
        <CardDescription>{t('home.cli.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <dl className="grid gap-x-6 gap-y-3 sm:grid-cols-[auto_1fr]">
          {GENWORK_COMMANDS.map((command) => (
            <div key={command} className="contents">
              <dt>
                <code className="rounded-md bg-muted px-2 py-1 font-mono text-sm whitespace-nowrap">{genworkCommandLine(command)}</code>
              </dt>
              <dd className="text-sm text-muted-foreground sm:pt-1">{t(`home.cli.commands.${command}`)}</dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}
