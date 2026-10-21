import { URGENCY_LEVELS, type Urgency } from './types'

export function urgencyRank(urgency: Urgency): number {
  return URGENCY_LEVELS.indexOf(urgency)
}

export function urgencyTranslationKey(urgency: Urgency): string {
  return `gentasks.urgency.${urgency}`
}

// isUrgencyShown hides the "none" level on cards, where it would only add noise.
export function isUrgencyShown(urgency: Urgency): boolean {
  return urgency !== 'none'
}
