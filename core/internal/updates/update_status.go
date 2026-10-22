// Package updates relays admins' update requests to the updater container, the only one with the Docker socket
// (rule 84). It names no module (rule 52) and only translates the updater's contract (design §9).
package updates

import "time"

type LatestRelease struct {
	Version     string    `json:"version"`
	NotesURL    string    `json:"notesUrl"`
	PublishedAt time.Time `json:"publishedAt"`
}

type UpdateFailure struct {
	Code       string `json:"code"`
	Step       string `json:"step"`
	RolledBack bool   `json:"rolledBack"`
}

// UpdaterStatus is what the updater's API answers; the updater owns these facts, core only reads them.
type UpdaterStatus struct {
	InstalledVersion string         `json:"installedVersion"`
	Updatable        bool           `json:"updatable"`
	Reason           string         `json:"reason"`
	Latest           *LatestRelease `json:"latest"`
	CheckedAt        *time.Time     `json:"checkedAt"`
	State            string         `json:"state"`
	TargetVersion    string         `json:"targetVersion"`
	Step             string         `json:"step"`
	StartedAt        *time.Time     `json:"startedAt"`
	FinishedAt       *time.Time     `json:"finishedAt"`
	Failure          *UpdateFailure `json:"failure"`
}
