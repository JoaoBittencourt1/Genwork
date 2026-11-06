import { useAuth } from '@/app/auth/components/AuthContext'
import { genagentsAbilities, type GenagentsAbilities } from '../domain/permissions'

export function useGenagentsAbilities(): GenagentsAbilities {
  const { can } = useAuth()
  return genagentsAbilities(can)
}
