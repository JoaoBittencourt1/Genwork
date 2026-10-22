import { useMutation, useQuery } from '@tanstack/react-query'
import { decideAuthorization, previewAuthorization } from '../datasource/oauth'
import type { AuthorizationDecision, AuthorizationRequest } from '../domain/authorizationRequest'

// The browser leaves Genwork for the client's redirect URI, which core built and validated.
function sendBrowserTo(redirectUrl: string) {
  window.location.assign(redirectUrl)
}

export function useAuthorizationPreview(request: AuthorizationRequest | null) {
  const preview = useQuery({
    queryKey: ['oauth', 'authorization-request', request],
    queryFn: async () => {
      const answer = await previewAuthorization(request!)
      if (answer.kind === 'redirect') sendBrowserTo(answer.redirectUrl)
      return answer
    },
    enabled: request !== null,
  })
  const decision = useMutation({
    mutationFn: (choice: AuthorizationDecision) => decideAuthorization(request!, choice),
    onSuccess: sendBrowserTo,
  })
  return { preview, decision }
}
