import type { ReactNode } from 'react'

type AlertTone = 'error' | 'success' | 'warning'

const toneClasses: Record<AlertTone, string> = {
  error: 'border-red-200 bg-red-50 text-red-800',
  success: 'border-green-200 bg-green-50 text-green-800',
  warning: 'border-amber-200 bg-amber-50 text-amber-800',
}

export function Alert({ tone, children }: { tone: AlertTone; children: ReactNode }) {
  return (
    <div role={tone === 'error' ? 'alert' : 'status'} className={`rounded-md border px-3 py-2 text-sm ${toneClasses[tone]}`}>
      {children}
    </div>
  )
}
