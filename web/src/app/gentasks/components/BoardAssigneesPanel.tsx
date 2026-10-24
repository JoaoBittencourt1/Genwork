import { useTranslation } from 'react-i18next'
import type { DirectoryUser } from '@/shared/datasource/users'
import { cn } from '@/shared/lib/utils'
import { Badge } from '@/shared/ui/badge'
import { Progress } from '@/shared/ui/progress'
import { summarizeBoardAssignees, summarizeBoardProgress, type BoardProgressSummary } from '../domain/assignees'
import { isAssigneeSelected, toggleAssigneeFilter, UNASSIGNED, type TaskFilter } from '../domain/filters'
import type { BoardView } from '../domain/types'
import { AssigneeAvatar } from './AssigneeAvatar'

type BoardAssigneesPanelProps = {
  board: BoardView
  users: DirectoryUser[] | undefined
  filter: TaskFilter
  onFilterChange: (filter: TaskFilter) => void
}

export function BoardAssigneesPanel({ board, users, filter, onFilterChange }: BoardAssigneesPanelProps) {
  const { t } = useTranslation()
  const unassignedLabel = t('gentasks.task.unassigned')
  const breakdown = summarizeBoardAssignees(board, users)
  const progress = summarizeBoardProgress(board, users)

  return (
    <aside
      aria-label={t('gentasks.assigneesPanel.title')}
      className="flex w-72 shrink-0 flex-col border-l bg-card/30"
    >
      <div className="flex h-10 shrink-0 items-center justify-between border-b px-4">
        <span className="text-xs font-medium text-muted-foreground">{t('gentasks.assigneesPanel.title')}</span>
        <Badge variant="secondary" className="tabular-nums">
          {t('gentasks.assigneesPanel.totalTasks', { count: breakdown.totalTasks })}
        </Badge>
      </div>
      <div className="flex-1 space-y-1 overflow-y-auto p-2">
        {breakdown.entries.map((entry) => {
          const key = entry.assigneeToken ?? UNASSIGNED
          const name = entry.user?.name ?? unassignedLabel
          const selected = isAssigneeSelected(filter, entry.assigneeToken)

          return (
            <button
              key={key}
              type="button"
              aria-pressed={selected}
              onClick={() => onFilterChange(toggleAssigneeFilter(filter, entry.assigneeToken))}
              className={cn(
                'flex w-full flex-col gap-1.5 rounded-md px-2.5 py-2 text-left text-xs transition-colors hover:bg-accent/60',
                selected && 'bg-accent text-accent-foreground',
              )}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="flex min-w-0 items-center gap-2">
                  <AssigneeAvatar assignee={entry.user} unassignedLabel={unassignedLabel} />
                  <span className={cn('truncate font-medium', !entry.user && 'text-muted-foreground')}>{name}</span>
                </span>
                <span className="flex shrink-0 items-center gap-1.5 tabular-nums">
                  <span className="text-muted-foreground">{entry.percentage}%</span>
                  <span className="min-w-5 text-right font-medium text-foreground">{entry.count}</span>
                </span>
              </div>
              <Progress value={entry.percentage} className="h-1" />
            </button>
          )
        })}
        <BoardProgressChart taskPrefix={board.taskPrefix} progress={progress} filter={filter} />
      </div>
    </aside>
  )
}

function BoardProgressChart({
  taskPrefix,
  progress,
  filter,
}: {
  taskPrefix: string
  progress: BoardProgressSummary
  filter: TaskFilter
}) {
  const { t } = useTranslation()
  const plotLeft = 30
  const plotRight = 230
  const plotTop = 12
  const plotBottom = 96
  const plotWidth = plotRight - plotLeft
  const plotHeight = plotBottom - plotTop
  const maxStep = Math.max(progress.totalTasks, 1)

  const toX = (step: number) => plotLeft + (step / maxStep) * plotWidth
  const toY = (percentage: number) => plotBottom - (percentage / 100) * plotHeight

  const totalCoordinates = progress.points.map((point) => ({
    ...point,
    x: toX(point.step),
    y: toY(point.percentage),
  }))

  const totalPolylinePoints = totalCoordinates.map((point) => `${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(' ')
  const lastTotalCoordinate = totalCoordinates.at(-1) ?? { x: plotLeft, y: plotBottom }
  const totalAreaPoints = `${plotLeft},${plotBottom} ${totalPolylinePoints} ${lastTotalCoordinate.x.toFixed(1)},${plotBottom}`
  const hasActiveAssigneeFilter = filter.assigneeTokens.length > 0

  return (
    <div className="px-2.5 pt-3 pb-2">
      <div className="mb-2 flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-muted-foreground">{t('gentasks.assigneesPanel.progressTitle')}</span>
        <span className="flex items-center gap-1.5 text-xs tabular-nums">
          <span className="text-muted-foreground">
            {t('gentasks.assigneesPanel.completedCount', {
              completed: progress.completedTasks,
              total: progress.totalTasks,
            })}
          </span>
          <Badge variant="secondary" className="px-1.5 py-0 text-[10px] tabular-nums">
            {progress.completionPercentage}%
          </Badge>
        </span>
      </div>

      <svg
        viewBox="0 0 240 116"
        role="img"
        aria-label={t('gentasks.assigneesPanel.progressTitle')}
        className="h-32 w-full overflow-visible"
      >
        {/* Horizontal grid lines (Y: 100% and 50%) */}
        <line
          x1={plotLeft}
          y1={plotTop}
          x2={plotRight}
          y2={plotTop}
          stroke="currentColor"
          strokeDasharray="2 2"
          className="text-border/70"
        />
        <line
          x1={plotLeft}
          y1={toY(50)}
          x2={plotRight}
          y2={toY(50)}
          stroke="currentColor"
          strokeDasharray="2 2"
          className="text-border/50"
        />

        {/* Cartesian Axes (Y and X) */}
        <line x1={plotLeft} y1={plotTop} x2={plotLeft} y2={plotBottom} stroke="currentColor" className="text-border" />
        <line x1={plotLeft} y1={plotBottom} x2={plotRight} y2={plotBottom} stroke="currentColor" className="text-border" />

        {/* Y-Axis Labels (0% to 100%) */}
        <text x={plotLeft - 5} y={plotTop + 3} textAnchor="end" className="fill-muted-foreground text-[9px] tabular-nums">
          100%
        </text>
        <text
          x={plotLeft - 5}
          y={toY(50) + 3}
          textAnchor="end"
          className="fill-muted-foreground text-[9px] tabular-nums"
        >
          50%
        </text>
        <text x={plotLeft - 5} y={plotBottom + 3} textAnchor="end" className="fill-muted-foreground text-[9px] tabular-nums">
          0%
        </text>

        {/* X-Axis Labels */}
        <text x={plotLeft} y={plotBottom + 13} textAnchor="start" className="fill-muted-foreground text-[9px] tabular-nums">
          0
        </text>
        <text x={(plotLeft + plotRight) / 2} y={plotBottom + 13} textAnchor="middle" className="fill-muted-foreground text-[9px]">
          {t('gentasks.assigneesPanel.axisX')}
        </text>
        <text x={plotRight} y={plotBottom + 13} textAnchor="end" className="fill-muted-foreground text-[9px] tabular-nums">
          {progress.totalTasks}
        </text>

        {/* Overall project progress area & dashed line */}
        {totalCoordinates.length > 1 ? <polygon points={totalAreaPoints} className="fill-primary/10" /> : null}
        <polyline
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeDasharray="3 2"
          strokeLinecap="round"
          strokeLinejoin="round"
          points={totalPolylinePoints}
          className="text-muted-foreground/70"
        />

        {/* Per-member lines showing % of project completed by each person */}
        {progress.assigneeSeries.map((series) => {
          const key = series.assigneeToken ?? UNASSIGNED
          const color = series.color
          const isSelected = isAssigneeSelected(filter, series.assigneeToken)
          const dimmed = hasActiveAssigneeFilter && !isSelected
          const seriesCoords = series.points.map((point) => ({
            ...point,
            x: toX(point.step),
            y: toY(point.percentage),
          }))
          const seriesPolyline = seriesCoords.map((point) => `${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(' ')

          return (
            <g key={key} className={cn('transition-opacity', dimmed ? 'opacity-25' : 'opacity-100')}>
              <polyline
                fill="none"
                stroke={color}
                strokeWidth={isSelected ? '2.5' : '1.75'}
                strokeLinecap="round"
                strokeLinejoin="round"
                points={seriesPolyline}
              />
              {seriesCoords.map((point) => {
                const isLast = point.step === progress.completedTasks
                if (point.taskNumber === null && !isLast) return null
                return (
                  <circle
                    key={point.step}
                    cx={point.x}
                    cy={point.y}
                    r={isLast ? 3 : 2}
                    style={{ fill: color }}
                    className="stroke-background stroke-1"
                  >
                    <title>
                      {point.taskNumber !== null
                        ? `${series.user?.name ?? ''} · ${taskPrefix}-${point.taskNumber}: ${point.title ?? ''} (${point.percentage}%)`
                        : `${series.user?.name ?? ''}: ${series.completionPercentage}% (${series.completedTasks}/${progress.totalTasks})`}
                    </title>
                  </circle>
                )
              })}
            </g>
          )
        })}
      </svg>

      {progress.assigneeSeries.length > 0 ? (
        <div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1">
          {progress.assigneeSeries.map((series) => {
            const key = series.assigneeToken ?? UNASSIGNED
            return (
              <span key={key} className="inline-flex items-center gap-1 text-[10px] text-muted-foreground tabular-nums">
                <span className="size-2 shrink-0 rounded-full" style={{ backgroundColor: series.color }} />
                <span className="max-w-20 truncate">{series.user?.name}</span>
                <span className="font-medium text-foreground">{series.completionPercentage}%</span>
              </span>
            )
          })}
        </div>
      ) : null}
    </div>
  )
}
