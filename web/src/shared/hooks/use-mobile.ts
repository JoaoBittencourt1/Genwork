import * as React from "react"

const MOBILE_BREAKPOINT = 768
const mobileQuery = `(max-width: ${MOBILE_BREAKPOINT - 1}px)`

function subscribeToViewport(onChange: () => void) {
  const mql = window.matchMedia(mobileQuery)
  mql.addEventListener("change", onChange)
  return () => mql.removeEventListener("change", onChange)
}

function isMobileViewport() {
  return window.matchMedia(mobileQuery).matches
}

export function useIsMobile() {
  return React.useSyncExternalStore(subscribeToViewport, isMobileViewport)
}
