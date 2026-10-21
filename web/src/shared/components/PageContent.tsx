import type { ReactNode } from 'react'
import { cn } from '@/shared/lib/utils'

// PageContent scrolls below the PageHeader; pages that need the full width, like a board, skip it.
export function PageContent({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className={cn('mx-auto w-full max-w-5xl px-4 py-8 md:px-8', className)}>{children}</div>
    </div>
  )
}
