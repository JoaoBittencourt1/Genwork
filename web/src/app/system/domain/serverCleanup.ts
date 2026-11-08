export type OutdatedImage = { reference: string; size: number }

// CleanupPreview is what the server can free: the images of earlier Genwork versions.
export type CleanupPreview = { images: OutdatedImage[]; totalBytes: number }

// CleanupReport is what was removed, and what stayed because a container still uses it.
export type CleanupReport = { removed: OutdatedImage[]; skipped: OutdatedImage[]; freedBytes: number }

export function canFreeSpace(preview: CleanupPreview | undefined): boolean {
  return (preview?.images.length ?? 0) > 0
}

// cleanupOutcomeKey picks the message after a cleanup: nothing to do, everything removed, or some images kept.
export function cleanupOutcomeKey(report: CleanupReport): 'system.cleanup.nothingRemoved' | 'system.cleanup.removed' | 'system.cleanup.removedWithSkipped' {
  if (report.removed.length === 0 && report.skipped.length === 0) return 'system.cleanup.nothingRemoved'
  return report.skipped.length > 0 ? 'system.cleanup.removedWithSkipped' : 'system.cleanup.removed'
}
