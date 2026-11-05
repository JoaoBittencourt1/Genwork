import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PlusIcon, Trash2Icon } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/components/ConfirmDialog'
import { ErrorAlert } from '@/shared/components/ErrorAlert'
import { useErrorMessage } from '@/shared/hooks/useErrorMessage'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/shared/ui/field'
import { Input } from '@/shared/ui/input'
import { Spinner } from '@/shared/ui/spinner'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/shared/ui/table'
import { Textarea } from '@/shared/ui/textarea'
import { createSkill, deleteSkill, listSkills, updateSkill } from '../datasource/genagentsDatasource'
import { addSkillFile, parseSkillMarkdown } from '../domain/skills'
import type { Skill, SkillFile } from '../domain/types'
import { genagentsQueryKeys } from './queryKeys'

export function SkillsSection({ canManage }: { canManage: boolean }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const skills = useQuery({ queryKey: genagentsQueryKeys.skills, queryFn: listSkills, enabled: canManage })
  const [editing, setEditing] = useState<Skill | 'new' | null>(null)
  const [removing, setRemoving] = useState<Skill | null>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: genagentsQueryKeys.skills })
  const remove = useMutation({ mutationFn: (skill: Skill) => deleteSkill(skill.token), onSuccess: refresh })

  if (skills.isPending) return <Spinner />
  if (skills.isLoadingError) return <ErrorAlert error={skills.error} />

  return (
    <div className="flex flex-col gap-3">
      <ErrorAlert error={remove.error} />
      <div className="rounded-lg border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('genagents.skills.columns.name')}</TableHead>
              <TableHead>{t('genagents.skills.columns.description')}</TableHead>
              <TableHead>{t('genagents.skills.columns.version')}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t('genagents.actions')}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {skills.data.map((skill) => (
              <TableRow key={skill.token}>
                <TableCell className="font-medium">{skill.name}</TableCell>
                <TableCell className="max-w-md truncate">{skill.description}</TableCell>
                <TableCell>{skill.version}</TableCell>
                <TableCell className="text-right">
                  {canManage ? (
                    <div className="flex justify-end gap-1">
                      <Button size="sm" variant="ghost" onClick={() => setEditing(skill)}>
                        {t('genagents.skills.edit')}
                      </Button>
                      <Button size="sm" variant="ghost" aria-label={t('genagents.skills.remove', { name: skill.name })} onClick={() => setRemoving(skill)}>
                        <Trash2Icon />
                      </Button>
                    </div>
                  ) : null}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {canManage ? (
        <div>
          <Button onClick={() => setEditing('new')}>
            <PlusIcon />
            {t('genagents.skills.add')}
          </Button>
        </div>
      ) : null}
      <Dialog open={editing !== null} onOpenChange={(open) => !open && setEditing(null)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          {editing !== null ? <SkillForm skill={editing === 'new' ? null : editing} onDone={() => setEditing(null)} onSaved={refresh} /> : null}
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={removing !== null}
        title={t('genagents.skills.removeTitle')}
        description={t('genagents.skills.removeDescription', { name: removing?.name ?? '' })}
        confirmLabel={t('common.delete')}
        onOpenChange={(open) => !open && setRemoving(null)}
        onConfirm={() => {
          if (removing) remove.mutate(removing)
          setRemoving(null)
        }}
      />
    </div>
  )
}

function SkillForm({ skill, onDone, onSaved }: { skill: Skill | null; onDone: () => void; onSaved: () => void }) {
  const { t } = useTranslation()
  const { describeFieldError } = useErrorMessage()
  const [name, setName] = useState(skill?.name ?? '')
  const [description, setDescription] = useState(skill?.description ?? '')
  const [body, setBody] = useState(skill?.body ?? '')
  const [files, setFiles] = useState<SkillFile[]>(skill?.files ?? [])
  const [newPath, setNewPath] = useState('')
  const save = useMutation({
    mutationFn: () => {
      const values = { name: name.trim(), description: description.trim(), body, files }
      return skill ? updateSkill(skill.token, values) : createSkill(values)
    },
    onSuccess: () => {
      onSaved()
      onDone()
    },
  })

  function pasteSkillMarkdown(text: string) {
    const parsed = parseSkillMarkdown(text)
    if (!parsed) return
    setName(parsed.name)
    setDescription(parsed.description)
    setBody(parsed.body)
  }

  function submit(event: FormEvent) {
    event.preventDefault()
    save.mutate()
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{skill ? t('genagents.skills.editTitle', { name: skill.name }) : t('genagents.skills.add')}</DialogTitle>
      </DialogHeader>
      <ErrorAlert error={save.error} />
      {skill ? null : (
        <Field>
          <FieldLabel htmlFor="skill-import">{t('genagents.skills.import')}</FieldLabel>
          <Textarea id="skill-import" rows={3} placeholder={t('genagents.skills.importPlaceholder')} onChange={(event) => pasteSkillMarkdown(event.target.value)} />
        </Field>
      )}
      <Field data-invalid={Boolean(describeFieldError(save.error, 'name'))}>
        <FieldLabel htmlFor="skill-name">{t('genagents.skills.columns.name')}</FieldLabel>
        <Input id="skill-name" value={name} onChange={(event) => setName(event.target.value)} />
        <FieldError>{describeFieldError(save.error, 'name')}</FieldError>
      </Field>
      <Field data-invalid={Boolean(describeFieldError(save.error, 'description'))}>
        <FieldLabel htmlFor="skill-description">{t('genagents.skills.columns.description')}</FieldLabel>
        <Input id="skill-description" value={description} onChange={(event) => setDescription(event.target.value)} />
        <FieldError>{describeFieldError(save.error, 'description')}</FieldError>
      </Field>
      <Field data-invalid={Boolean(describeFieldError(save.error, 'body'))}>
        <FieldLabel htmlFor="skill-body">{t('genagents.skills.body')}</FieldLabel>
        <Textarea id="skill-body" rows={8} value={body} onChange={(event) => setBody(event.target.value)} />
        <FieldError>{describeFieldError(save.error, 'body')}</FieldError>
      </Field>
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">{t('genagents.skills.files')}</legend>
        {files.map((file, index) => (
          <div key={file.path} className="flex flex-col gap-1">
            <div className="flex items-center justify-between text-sm">
              <span className="font-mono">{file.path}</span>
              <Button type="button" size="sm" variant="ghost" aria-label={t('genagents.skills.removeFile', { path: file.path })} onClick={() => setFiles(files.filter((_, position) => position !== index))}>
                <Trash2Icon />
              </Button>
            </div>
            <Textarea rows={3} value={file.content} aria-label={file.path} onChange={(event) => setFiles(files.map((existing, position) => (position === index ? { ...existing, content: event.target.value } : existing)))} />
          </div>
        ))}
        <div className="flex gap-2">
          <Input value={newPath} aria-label={t('genagents.skills.newFilePath')} placeholder="examples/handler.go" onChange={(event) => setNewPath(event.target.value)} />
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setFiles(addSkillFile(files, newPath))
              setNewPath('')
            }}
          >
            {t('genagents.skills.addFile')}
          </Button>
        </div>
        <FieldError>{describeFieldError(save.error, 'files')}</FieldError>
      </fieldset>
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </Button>
      </DialogFooter>
    </form>
  )
}
