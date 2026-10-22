import { useTranslation } from 'react-i18next'
import type { Role } from '@/shared/domain/role'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/ui/select'
import type { Member } from '../domain/member'

type MemberRoleSelectProps = {
  member: Member
  roles: Role[]
  disabled: boolean
  onChange: (roleToken: string) => void
}

export function MemberRoleSelect({ member, roles, disabled, onChange }: MemberRoleSelectProps) {
  const { t } = useTranslation()
  return (
    <Select value={member.role.token} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger size="sm" className="w-32" aria-label={t('members.roleFor', { name: member.name })}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {roles.map((role) => (
          <SelectItem key={role.token} value={role.token}>
            {role.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
