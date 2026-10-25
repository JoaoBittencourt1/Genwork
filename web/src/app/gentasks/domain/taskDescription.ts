export const MAX_DESCRIPTION_LENGTH = 10000

export function isDescriptionBlank(description: string): boolean {
  return description.trim() === ''
}
