export const LIVE_REFRESH_INTERVAL_MS = 10_000

export const NO_LIVE_REFRESH = { refetchInterval: false, refetchOnWindowFocus: false } as const

// A refresh that lands while an optimistic change is being saved would briefly show the server's previous state.
export function liveRefreshInterval({ savingChanges }: { savingChanges: boolean }): number | false {
  return savingChanges ? false : LIVE_REFRESH_INTERVAL_MS
}
