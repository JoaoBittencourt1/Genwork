import { describe, expect, it } from 'vitest'
import { isPatchEmpty } from './patch'
import { boardViewFixture, bugLabel, featureLabel, taskFixture } from '@/test/gentasksFixtures'
import {
  buildTaskPatch,
  draftFromTask,
  emptyTaskDraft,
  isDraftTitleValid,
  newTaskFromDraft,
  resolveColumnChange,
} from './taskPatch'

describe('buildTaskPatch', () => {
  const task = taskFixture({ description: 'Body', assigneeToken: 'user-ana', labels: [bugLabel, featureLabel] })

  it('sends nothing when nothing changed', () => {
    const patch = buildTaskPatch(task, draftFromTask(task))
    expect(patch).toEqual({})
    expect(isPatchEmpty(patch)).toBe(true)
  })

  it('leaves the column out of the patch', () => {
    expect(buildTaskPatch(task, { ...draftFromTask(task), columnToken: 'column-done' })).toEqual({})
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
  const draft = emptyTaskDraft('column-todo')

  it('validates the title length', () => {
    expect(isDraftTitleValid({ ...draft, title: '   ' })).toBe(false)
    expect(isDraftTitleValid({ ...draft, title: 'a'.repeat(201) })).toBe(false)
    expect(isDraftTitleValid({ ...draft, title: 'Ship' })).toBe(true)
  })

  it('omits empty optional fields when creating', () => {
    expect(newTaskFromDraft({ ...draft, title: ' Ship ' })).toEqual({ title: 'Ship', columnToken: 'column-todo', urgency: 'none' })
    expect(
      newTaskFromDraft({
        title: 'Ship',
        description: 'Body',
        columnToken: 'c',
        assigneeToken: 'user-ana',
        urgency: 'high',
        labelTokens: ['label-bug'],
      }),
    ).toEqual({ title: 'Ship', columnToken: 'c', urgency: 'high', description: 'Body', assigneeToken: 'user-ana', labelTokens: ['label-bug'] })
  })
})

describe('resolveColumnChange', () => {
  const view = boardViewFixture()
  const task = view.columns[0]!.tasks[0]!

  it('moves a task whose column changed to the end of the new column', () => {
    expect(resolveColumnChange(view, task, { ...draftFromTask(task), columnToken: 'column-todo' })).toEqual({
      taskToken: task.token,
      toColumnToken: 'column-todo',
      toPosition: 1,
    })
  })

  it('does nothing when the column is unchanged or unknown', () => {
    expect(resolveColumnChange(view, task, draftFromTask(task))).toBeUndefined()
    expect(resolveColumnChange(view, task, { ...draftFromTask(task), columnToken: 'missing' })).toBeUndefined()
  })
})
