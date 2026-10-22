import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { moduleNameTranslationKey } from '@/shared/domain/capabilities'
import { Button } from '@/shared/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/shared/ui/dialog'
import { Label } from '@/shared/ui/label'
import { Switch } from '@/shared/ui/switch'
import { moduleAccessByModule, toRestrictedModules, type Member } from '../domain/member'

type MemberModuleAccessDialogProps = {
  member: Member | null
  moduleIds: string[]
  saving: boolean
  onSave: (member: Member, restrictedModules: string[]) => void
  onClose: () => void
}

export function MemberModuleAccessDialog({ member, moduleIds, saving, onSave, onClose }: MemberModuleAccessDialogProps) {
  return (
    <Dialog open={member !== null} onOpenChange={(open) => (open ? undefined : onClose())}>
      <DialogContent>
        {member ? <ModuleAccessForm key={member.token} member={member} moduleIds={moduleIds} saving={saving} onSave={onSave} onClose={onClose} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function ModuleAccessForm({ member, moduleIds, saving, onSave, onClose }: MemberModuleAccessDialogProps & { member: Member }) {
  const { t } = useTranslation()
  const [accessByModule, setAccessByModule] = useState(() => moduleAccessByModule(moduleIds, member.restrictedModules))

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t('members.modules.dialogTitle', { name: member.name })}</DialogTitle>
        <DialogDescription>{t('members.modules.dialogDescription')}</DialogDescription>
      </DialogHeader>
      {moduleIds.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('members.modules.noneEnabled')}</p>
      ) : (
        <ul className="flex flex-col gap-3">
          {moduleIds.map((moduleId) => (
            <li key={moduleId} className="flex items-center justify-between gap-4">
              <Label htmlFor={`module-access-${moduleId}`}>{t(moduleNameTranslationKey(moduleId), { defaultValue: moduleId })}</Label>
              <Switch
                id={`module-access-${moduleId}`}
                checked={accessByModule[moduleId] ?? true}
                onCheckedChange={(checked) => setAccessByModule({ ...accessByModule, [moduleId]: checked })}
              />
            </li>
          ))}
        </ul>
      )}
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          {t('common.cancel')}
        </Button>
        <Button disabled={saving || moduleIds.length === 0} onClick={() => onSave(member, toRestrictedModules(member.restrictedModules, moduleIds, accessByModule))}>
          {saving ? t('common.saving') : t('common.save')}
        </Button>
      </DialogFooter>
    </>
  )
}
