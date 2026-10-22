// isReachableFromInternet is false when a service on the internet (GitHub, claude.ai) cannot reach the URL: not https,
// or a local or private host.
export function isReachableFromInternet(url: string): boolean {
  let parsed: URL
  try {
    parsed = new URL(url)
  } catch {
    return false
  }
  if (parsed.protocol !== 'https:') return false
  const host = parsed.hostname.toLowerCase()
  if (host.startsWith('[')) return !isLocalIPv6(host.slice(1, -1))
  if (host === 'localhost' || host.endsWith('.localhost')) return false
  return !isLocalIPv4(host)
}

function isLocalIPv4(host: string): boolean {
  const octets = host.split('.').map(Number)
  if (octets.length !== 4 || octets.some((octet) => !Number.isInteger(octet))) return false
  const [first, second] = octets as [number, number, number, number]
  return (
    first === 0 ||
    first === 10 ||
    first === 127 ||
    (first === 169 && second === 254) ||
    (first === 172 && second >= 16 && second <= 31) ||
    (first === 192 && second === 168)
  )
}

// isLocalIPv6 covers loopback, unique local (fc00::/7), and link-local (fe80::/10) addresses.
function isLocalIPv6(host: string): boolean {
  return host === '::1' || /^f[cd]/.test(host) || /^fe[89ab]/.test(host)
}
