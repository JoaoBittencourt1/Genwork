const SECOND = 1000
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR
const MONTH = 30 * DAY
const YEAR = 365 * DAY

const UNITS: { unit: Intl.RelativeTimeFormatUnit; milliseconds: number }[] = [
  { unit: 'year', milliseconds: YEAR },
  { unit: 'month', milliseconds: MONTH },
  { unit: 'day', milliseconds: DAY },
  { unit: 'hour', milliseconds: HOUR },
  { unit: 'minute', milliseconds: MINUTE },
]

export function formatTimeAgo(timestamp: string, now: Date, language: string): string {
  const elapsed = Math.max(0, now.getTime() - new Date(timestamp).getTime())
  const format = new Intl.RelativeTimeFormat(language, { numeric: 'auto' })
  const fitting = UNITS.find(({ milliseconds }) => elapsed >= milliseconds)
  if (!fitting) return format.format(0, 'second')
  return format.format(-Math.floor(elapsed / fitting.milliseconds), fitting.unit)
}
