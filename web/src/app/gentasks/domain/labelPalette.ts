import type { LabelColor, Urgency } from './types'

// Colors come only from theme variables defined in index.css (rule 81).
export function labelColorVariable(color: LabelColor): string {
  return `var(--label-${color})`
}

export function urgencyColorVariable(urgency: Urgency): string {
  return `var(--urgency-${urgency})`
}

export function labelColorTranslationKey(color: LabelColor): string {
  return `gentasks.labelColors.${color}`
}
