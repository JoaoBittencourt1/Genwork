package updates

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type UpdateState string

const (
	StateIdle      UpdateState = "idle"
	StateUpdating  UpdateState = "updating"
	StateSucceeded UpdateState = "succeeded"
	StateFailed    UpdateState = "failed"
)

type UpdateStep string

const (
	StepDownload UpdateStep = "download"
	StepPull     UpdateStep = "pull"
	StepBackup   UpdateStep = "backup"
	StepSwap     UpdateStep = "swap"
	StepMigrate  UpdateStep = "migrate"
	StepVerify   UpdateStep = "verify"
)

type LatestRelease struct {
	Version     string    `json:"version"`
	NotesURL    string    `json:"notesUrl"`
	PublishedAt time.Time `json:"publishedAt"`
}

type UpdateFailure struct {
	Code       string     `json:"code"`
	Step       UpdateStep `json:"step"`
	RolledBack bool       `json:"rolledBack"`
	// BackupPath is set when the rollback failed, so the operator can restore the database by hand.
	BackupPath string `json:"backupPath,omitempty"`
}

// RecordedStatus is deploy/update-status.json, the single owner of the update status (rule 7): the CLI prints it and
// the updater serves it, so it survives the updater's own restart.
type RecordedStatus struct {
	Latest          *LatestRelease `json:"latest"`
	CheckedAt       *time.Time     `json:"checkedAt"`
	State           UpdateState    `json:"state"`
	TargetVersion   string         `json:"targetVersion,omitempty"`
	PreviousVersion string         `json:"previousVersion,omitempty"`
	Step            UpdateStep     `json:"step,omitempty"`
	StartedAt       *time.Time     `json:"startedAt,omitempty"`
	FinishedAt      *time.Time     `json:"finishedAt,omitempty"`
	Failure         *UpdateFailure `json:"failure,omitempty"`
}

// UpdateStatus adds what VERSION and the workspace kind own to the recorded status.
type UpdateStatus struct {
	InstalledVersion string `json:"installedVersion"`
	Updatable        bool   `json:"updatable"`
	Reason           string `json:"reason,omitempty"`
	RecordedStatus
}

type statusStore struct {
	path  string
	owner installationOwner
}

func (store statusStore) read() (RecordedStatus, error) {
	raw, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return RecordedStatus{State: StateIdle}, nil
	}
	if err != nil {
		return RecordedStatus{}, fmt.Errorf("read %s: %w", store.path, err)
	}
	var recorded RecordedStatus
	if err := json.Unmarshal(raw, &recorded); err != nil {
		return RecordedStatus{}, fmt.Errorf("parse %s: %w", store.path, err)
	}
	return recorded, nil
}

// write replaces the file atomically, so a reader never sees half a status.
func (store statusStore) write(recorded RecordedStatus) error {
	body, err := json.MarshalIndent(recorded, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", store.path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(store.path), ".update-status-*.json")
	if err != nil {
		return fmt.Errorf("write %s: %w", store.path, err)
	}
	defer os.Remove(temporary.Name())
	if _, err := temporary.Write(append(body, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", store.path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", store.path, err)
	}
	if err := store.owner.claim(temporary.Name()); err != nil {
		return fmt.Errorf("write %s: %w", store.path, err)
	}
	if err := os.Rename(temporary.Name(), store.path); err != nil {
		return fmt.Errorf("write %s: %w", store.path, err)
	}
	return nil
}
