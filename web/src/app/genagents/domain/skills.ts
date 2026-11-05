import type { SkillFile } from './types'

export const MAX_SKILL_FILES = 20
export const MAX_SKILL_NAME_LENGTH = 64
const SKILL_NAME_PATTERN = /^[a-z0-9-]{1,64}$/

export function isValidSkillName(name: string): boolean {
  return SKILL_NAME_PATTERN.test(name)
}

// isValidSkillFilePath mirrors the API: relative, clean, and never leaving the skill directory.
export function isValidSkillFilePath(path: string): boolean {
  if (path === '' || path === 'SKILL.md' || path.startsWith('/') || path.includes('\\')) return false
  return path.split('/').every((segment) => segment !== '' && segment !== '.' && segment !== '..')
}

export function addSkillFile(files: SkillFile[], path: string): SkillFile[] {
  const trimmed = path.trim()
  if (!isValidSkillFilePath(trimmed) || files.length >= MAX_SKILL_FILES || files.some((file) => file.path === trimmed)) return files
  return [...files, { path: trimmed, content: '' }]
}

// parseSkillMarkdown reads the front matter of a pasted SKILL.md so the form can fill name and description.
export function parseSkillMarkdown(content: string): { name: string; description: string; body: string } | null {
  const normalized = content.replace(/\r\n/g, '\n')
  if (!normalized.startsWith('---\n')) return null
  const end = normalized.indexOf('\n---', 4)
  if (end === -1) return null
  let name = ''
  let description = ''
  for (const line of normalized.slice(4, end).split('\n')) {
    const separator = line.indexOf(':')
    if (separator === -1) continue
    const value = line.slice(separator + 1).trim().replace(/^["']|["']$/g, '')
    const key = line.slice(0, separator).trim()
    if (key === 'name') name = value
    if (key === 'description') description = value
  }
  if (name === '' || description === '') return null
  return { name, description, body: normalized.slice(end + 4).replace(/^\n+/, '') }
}
