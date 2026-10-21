import { ChevronDownIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { Button } from '@/shared/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/shared/ui/dropdown-menu'
import { EMPTY_FILTER, isFilterActive, toggleValue, UNASSIGNED, type TaskFilter } from '../domain/filters'
import { URGENCY_LEVELS, type Label } from '../domain/types'
import { urgencyTranslationKey } from '../domain/urgency'
import { LabelChip } from './LabelChip'

type BoardFiltersProps = {
  filter: TaskFilter
  labels: Label[]
  users: DirectoryUser[] | undefined
  onChange: (filter: TaskFilter) => void
}

export function BoardFilters({ filter, labels, users, onChange }: BoardFiltersProps) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-wrap items-center gap-2">
      <FilterMenu label={t('gentasks.filters.urgency')} count={filter.urgencies.length}>
        {URGENCY_LEVELS.map((urgency) => (
          <FilterItem
            key={urgency}
            checked={filter.urgencies.includes(urgency)}
            onToggle={() => onChange({ ...filter, urgencies: toggleValue(filter.urgencies, urgency) })}
          >
            {t(urgencyTranslationKey(urgency))}
          </FilterItem>
        ))}
      </FilterMenu>
      <FilterMenu label={t('gentasks.filters.labels')} count={filter.labelTokens.length} disabled={labels.length === 0}>
        {labels.map((label) => (
          <FilterItem
            key={label.token}
            checked={filter.labelTokens.includes(label.token)}
            onToggle={() => onChange({ ...filter, labelTokens: toggleValue(filter.labelTokens, label.token) })}
          >
            <LabelChip label={label} />
          </FilterItem>
        ))}
      </FilterMenu>
      <FilterMenu label={t('gentasks.filters.assignee')} count={filter.assigneeTokens.length}>
        {[{ token: UNASSIGNED, name: t('gentasks.task.unassigned') }, ...(users ?? [])].map((user) => (
          <FilterItem
            key={user.token}
            checked={filter.assigneeTokens.includes(user.token)}
            onToggle={() => onChange({ ...filter, assigneeTokens: toggleValue(filter.assigneeTokens, user.token) })}
          >
            {user.name}
          </FilterItem>
        ))}
      </FilterMenu>
      {isFilterActive(filter) ? (
        <Button variant="ghost" size="sm" onClick={() => onChange(EMPTY_FILTER)}>
          {t('gentasks.filters.clear')}
        </Button>
      ) : null}
    </div>
  )
}

function FilterMenu({ label, count, disabled, children }: { label: string; count: number; disabled?: boolean; children: ReactNode }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild disabled={disabled}>
        <Button variant={count > 0 ? 'secondary' : 'outline'} size="sm">
          {count > 0 ? `${label} · ${count}` : label}
          <ChevronDownIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">{children}</DropdownMenuContent>
    </DropdownMenu>
  )
}

function FilterItem({ checked, onToggle, children }: { checked: boolean; onToggle: () => void; children: ReactNode }) {
  return (
    <DropdownMenuCheckboxItem checked={checked} onSelect={(event) => event.preventDefault()} onCheckedChange={onToggle}>
      {children}
    </DropdownMenuCheckboxItem>
  )
}
