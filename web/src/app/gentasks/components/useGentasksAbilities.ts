import { useAuth } from '@/app/auth/components/AuthContext'
import { gentasksAbilities, type GentasksAbilities } from '../domain/permissions'

export function useGentasksAbilities(): GentasksAbilities {
  const { can } = useAuth()
  return gentasksAbilities(can)
}
