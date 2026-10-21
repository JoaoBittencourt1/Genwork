import { describe, expect, it } from 'vitest'
import { bugLabel, featureLabel, taskFixture } from '@/test/gentasksFixtures'
import { buildTaskPatch, draftFromTask, EMPTY_TASK_DRAFT, isDraftTitleValid, isPatchEmpty, newTaskFromDraft } from './taskPatch'

describe('buildTaskPatch', () => {
  const task = taskFixture({ description: 'Body', assigneeToken: 'user-ana', labels: [bugLabel, featureLabel] })

  it('sends nothing when nothing changed', () => {
    const patch = buildTaskPatch(task, draftFromTask(task))
    expect(patch).toEqual({})
    expect(isPatchEmpty(patch)).toBe(true)
  })

  it('sends only the changed urgency', () => {
    expect(buildTaskPatch(task, { ...draftFromTask(task), urgency: 'high' })).toEqual({ urgency: 'high' })
  })

  it('clears description and assignee with null', () => {
    expect(buildTaskPatch(task, { ...draftFromTask(task), description: '', assigneeToken: null })).toEqual({
      description: null,
      assigneeToken: null,
    })
  })

  it('trims the title and compares labels as a set', () => {
    const draft = { ...draftFromTask(task), title: '  Renamed  ', labelTokens: ['label-feature', 'label-bug'] }
    expect(buildTaskPatch(task, draft)).toEqual({ title: 'Renamed' })
    expect(buildTaskPatch(task, { ...draftFromTask(task), labelTokens: ['label-bug'] })).toEqual({ labelTokens: ['label-bug'] })
  })
})

describe('task drafts', () => {
  it('validates the title length', () => {
    expect(isDraftTitleValid({ ...EMPTY_TASK_DRAFT, title: '   ' })).toBe(false)
    expect(isDraftTitleValid({ ...EMPTY_TASK_DRAFT, title: 'a'.repeat(201) })).toBe(false)
    expect(isDraftTitleValid({ ...EMPTY_TASK_DRAFT, title: 'Ship' })).toBe(true)
  })

  it('omits empty optional fields when creating', () => {
    expect(newTaskFromDraft({ ...EMPTY_TASK_DRAFT, title: ' Ship ' }, 'column-todo')).toEqual({
      title: 'Ship',
      columnToken: 'column-todo',
      urgency: 'none',
    })
    expect(
      newTaskFromDraft({ title: 'Ship', description: 'Body', assigneeToken: 'user-ana', urgency: 'high', labelTokens: ['label-bug'] }, 'c'),
    ).toEqual({ title: 'Ship', columnToken: 'c', urgency: 'high', description: 'Body', assigneeToken: 'user-ana', labelTokens: ['label-bug'] })
  })
})
