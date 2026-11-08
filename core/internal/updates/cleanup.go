package updates

// OutdatedImage is an image of an earlier Genwork version the updater may remove, as it reports it.
type OutdatedImage struct {
	Reference string `json:"reference"`
	Size      int64  `json:"size"`
}

// CleanupPreview is what the updater would remove.
type CleanupPreview struct {
	Images     []OutdatedImage `json:"images"`
	TotalBytes int64           `json:"totalBytes"`
}

// CleanupReport is what the updater removed, and what it kept because a container still used it.
type CleanupReport struct {
	Removed    []OutdatedImage `json:"removed"`
	Skipped    []OutdatedImage `json:"skipped"`
	FreedBytes int64           `json:"freedBytes"`
}
