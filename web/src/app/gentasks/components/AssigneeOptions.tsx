import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { assigneeGroups } from '../domain/botAssignees'
import { useBotAssignment } from './BotAssignmentContext'

// useAssigneeGroups splits the directory into people and the bots a task may go to, for one task's current assignee.
export function useAssigneeGroups(users: DirectoryUser[], currentAssigneeToken: string | null) {
  const { botOptions, genagentsEnabled } = useBotAssignment()
  return assigneeGroups({ users, botOptions, genagentsEnabled, currentAssigneeToken })
}

export function useBotsGroupLabel(): string {
  const { t } = useTranslation()
  return t('gentasks.task.botsGroup')
}
