const BYTE_UNITS = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte', 'petabyte'] as const
const BYTES_PER_UNIT = 1024

// formatBytes shows a size in the largest binary unit below it, in the reader's locale.
export function formatBytes(bytes: number, locale: string): string {
  return formatInByteUnits(bytes, locale, '')
}

export function formatBytesPerSecond(bytesPerSecond: number, locale: string): string {
  return formatInByteUnits(bytesPerSecond, locale, '-per-second')
}

function formatInByteUnits(bytes: number, locale: string, unitSuffix: string): string {
  const exponent = bytes < BYTES_PER_UNIT ? 0 : Math.min(BYTE_UNITS.length - 1, Math.floor(Math.log(bytes) / Math.log(BYTES_PER_UNIT)))
  return new Intl.NumberFormat(locale, {
    style: 'unit',
    unit: `${BYTE_UNITS[exponent]}${unitSuffix}`,
    unitDisplay: 'short',
    maximumFractionDigits: exponent === 0 ? 0 : 1,
  }).format(bytes / BYTES_PER_UNIT ** exponent)
}
