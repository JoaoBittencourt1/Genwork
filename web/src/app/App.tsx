import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, type RouterHistory } from '@tanstack/react-router'
import { useState } from 'react'
import { AuthProvider } from '../core/auth/AuthProvider'
import { createAppRouter } from './router'

export function App({ history }: { history?: RouterHistory }) {
  const [queryClient] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } }))
  const [router] = useState(() => createAppRouter(history))
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
      </AuthProvider>
    </QueryClientProvider>
  )
}
