import { ListFilterIcon, XIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { EMPTY_FILTER, isFilterActive, selectedNames, toggleValue, UNASSIGNED, type TaskFilter } from '../domain/filters'
import { URGENCY_LEVELS, type Label } from '../domain/types'
import { urgencyTranslationKey } from '../domain/urgency'
import { LabelChip } from './LabelChip'
import { UrgencyIcon } from './UrgencyIcon'

type BoardFiltersProps = {
  filter: TaskFilter
  labels: Label[]
  users: DirectoryUser[] | undefined
  onChange: (filter: TaskFilter) => void
}

export function BoardFilterMenu({ filter, labels, users, onChange }: BoardFiltersProps) {
  const { t } = useTranslation()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant={isFilterActive(filter) ? 'secondary' : 'ghost'} size="sm">
          <ListFilterIcon />
          {t('gentasks.filters.title')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>{t('gentasks.filters.urgency')}</DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {URGENCY_LEVELS.map((urgency) => (
              <FilterItem
                key={urgency}
                checked={filter.urgencies.includes(urgency)}
                onToggle={() => onChange({ ...filter, urgencies: toggleValue(filter.urgencies, urgency) })}
              >
                <UrgencyIcon urgency={urgency} />
                {t(urgencyTranslationKey(urgency))}
              </FilterItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger disabled={labels.length === 0}>{t('gentasks.filters.labels')}</DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {labels.map((label) => (
              <FilterItem
                key={label.token}
                checked={filter.labelTokens.includes(label.token)}
                onToggle={() => onChange({ ...filter, labelTokens: toggleValue(filter.labelTokens, label.token) })}
              >
                <LabelChip label={label} />
              </FilterItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>{t('gentasks.filters.assignee')}</DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            {[{ token: UNASSIGNED, name: t('gentasks.task.unassigned') }, ...(users ?? [])].map((user) => (
              <FilterItem
                key={user.token}
                checked={filter.assigneeTokens.includes(user.token)}
                onToggle={() => onChange({ ...filter, assigneeTokens: toggleValue(filter.assigneeTokens, user.token) })}
              >
                {user.name}
              </FilterItem>
            ))}
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

// ActiveFilterBar lists the active filter dimensions as removable chips, under the board header.
export function ActiveFilterBar({ filter, labels, users, onChange }: BoardFiltersProps) {
  const { t } = useTranslation()
  if (!isFilterActive(filter)) return null
  const assigneeOptions = [{ token: UNASSIGNED, name: t('gentasks.task.unassigned') }, ...(users ?? [])]

  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2 border-b px-4 py-2">
      {filter.urgencies.length > 0 ? (
        <FilterChip label={t('gentasks.filters.urgency')} onRemove={() => onChange({ ...filter, urgencies: [] })}>
          {filter.urgencies.map((urgency) => t(urgencyTranslationKey(urgency))).join(', ')}
        </FilterChip>
      ) : null}
      {filter.labelTokens.length > 0 ? (
        <FilterChip label={t('gentasks.filters.labels')} onRemove={() => onChange({ ...filter, labelTokens: [] })}>
          {selectedNames(filter.labelTokens, labels)}
        </FilterChip>
      ) : null}
      {filter.assigneeTokens.length > 0 ? (
        <FilterChip label={t('gentasks.filters.assignee')} onRemove={() => onChange({ ...filter, assigneeTokens: [] })}>
          {selectedNames(filter.assigneeTokens, assigneeOptions)}
        </FilterChip>
      ) : null}
      <Button variant="ghost" size="xs" onClick={() => onChange(EMPTY_FILTER)}>
        {t('gentasks.filters.clear')}
      </Button>
      <p className="ml-auto text-xs text-muted-foreground">{t('gentasks.filters.dragDisabled')}</p>
    </div>
  )
}

function FilterChip({ label, onRemove, children }: { label: string; onRemove: () => void; children: ReactNode }) {
  const { t } = useTranslation()
  return (
    <span className="inline-flex h-7 items-center gap-1.5 rounded-md border bg-card pl-2 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span className="max-w-60 truncate font-medium">{children}</span>
      <button
        type="button"
        className="flex h-full items-center rounded-r-md border-l px-1.5 text-muted-foreground hover:text-foreground"
        aria-label={t('gentasks.filters.remove', { label })}
        onClick={onRemove}
      >
        <XIcon className="size-3" />
      </button>
    </span>
  )
}

function FilterItem({ checked, onToggle, children }: { checked: boolean; onToggle: () => void; children: ReactNode }) {
  return (
    <DropdownMenuCheckboxItem checked={checked} onSelect={(event) => event.preventDefault()} onCheckedChange={onToggle}>
      {children}
    </DropdownMenuCheckboxItem>
  )
}
