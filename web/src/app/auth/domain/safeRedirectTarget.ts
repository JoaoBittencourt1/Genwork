const HOME_PATH = '/'
const LOGIN_PATH = '/login'
const PROBE_ORIGIN = 'https://genwork.invalid'

// safeRedirectTarget accepts only a path on this origin, starting with a single "/": anything else, such as
// //evil.example.com, an absolute URL, or a backslash browsers read as "/", would make the login an open redirect.
export function safeRedirectTarget(raw: string | null | undefined): string {
  if (!raw || !raw.startsWith('/') || raw.startsWith('//') || hasBackslashOrControlCharacter(raw)) return HOME_PATH
  let parsed: URL
  try {
    parsed = new URL(raw, PROBE_ORIGIN)
  } catch {
    return HOME_PATH
  }
  if (parsed.origin !== PROBE_ORIGIN || parsed.pathname === LOGIN_PATH) return HOME_PATH
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

function hasBackslashOrControlCharacter(value: string): boolean {
  return [...value].some((character) => character === '\\' || character.charCodeAt(0) < 0x20)
}

// loginRedirectSearch is the login page's search for a page that needs a signed-in user.
export function loginRedirectSearch(returnPath: string): { redirect?: string } {
  return returnPath === HOME_PATH ? {} : { redirect: returnPath }
}
