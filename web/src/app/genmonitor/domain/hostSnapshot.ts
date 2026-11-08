export type StorageUsage = { totalBytes: number; usedBytes: number; availableBytes: number }

export type NetworkThroughput = {
  interface: string
  receivedBytesPerSecond: number
  transmittedBytesPerSecond: number
  receivedBytes: number
  transmittedBytes: number
}

export type HostSnapshot = {
  sampledAt: string
  sampleIntervalSeconds: number
  uptimeSeconds: number
  operatingSystem: { name: string; kernelRelease: string; hostname: string }
  cpu: { logicalCpus: number; usedPercent: number }
  memory: StorageUsage
  disk: StorageUsage
  network: NetworkThroughput
}

export type UsageLevel = 'normal' | 'high'

export const HIGH_USAGE_PERCENT = 90

// Until a snapshot arrives there is no server pace to follow; this is how long to wait before asking again.
export const RETRY_INTERVAL_MS = 5000


export function refreshIntervalMs(snapshot: HostSnapshot | undefined): number {
  return snapshot ? snapshot.sampleIntervalSeconds * 1000 : RETRY_INTERVAL_MS
}

// usedPercent divides by used plus available, like df: blocks reserved for root count as neither.
export function usedPercent(usage: StorageUsage): number {
  const usable = usage.usedBytes + usage.availableBytes
  if (usable <= 0) return 0
  return (usage.usedBytes / usable) * 100
}

export function usageLevel(percent: number): UsageLevel {
  return percent >= HIGH_USAGE_PERCENT ? 'high' : 'normal'
}

export function formatPercent(percent: number, locale: string): string {
  return new Intl.NumberFormat(locale, { style: 'percent', maximumFractionDigits: 1 }).format(percent / 100)
}

export type UptimePart = { unit: 'days' | 'hours' | 'minutes'; count: number }

// uptimeParts drops leading zero units, so a host up for minutes shows only minutes; minutes always show.
export function uptimeParts(uptimeSeconds: number): UptimePart[] {
  const totalMinutes = Math.floor(Math.max(0, uptimeSeconds) / 60)
  const parts: UptimePart[] = [
    { unit: 'days', count: Math.floor(totalMinutes / (24 * 60)) },
    { unit: 'hours', count: Math.floor(totalMinutes / 60) % 24 },
    { unit: 'minutes', count: totalMinutes % 60 },
  ]
  const firstShown = parts.findIndex((part) => part.count > 0 || part.unit === 'minutes')
  return parts.slice(firstShown)
}
