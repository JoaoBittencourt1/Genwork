import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { useState } from 'react'
import { AuthProvider } from '@/app/auth/components/AuthProvider'
import { ThemeProvider } from '@/shared/components/ThemeProvider'
import { LIVE_REFRESH_INTERVAL_MS } from '@/shared/domain/liveRefresh'
import { Toaster } from '@/shared/ui/sonner'
import { TooltipProvider } from '@/shared/ui/tooltip'
import { createAppRouter } from './router'

export function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { retry: false, refetchInterval: LIVE_REFRESH_INTERVAL_MS, refetchOnWindowFocus: true } },
      }),
  )
  const [router] = useState(() => createAppRouter())
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <TooltipProvider>
            <RouterProvider router={router} />
          </TooltipProvider>
          <Toaster />
        </AuthProvider>
      </QueryClientProvider>
    </ThemeProvider>
  )
}
