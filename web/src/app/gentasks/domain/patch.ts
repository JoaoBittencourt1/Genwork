// isPatchEmpty: a PATCH body without keys changes nothing, so callers can skip the request.
export function isPatchEmpty(patch: object): boolean {
  return Object.keys(patch).length === 0
}
