package updates

import "time"

// UpdateStatusResponse is the contract of the web app's update card; server details such as backup paths stay out.
type UpdateStatusResponse struct {
	InstalledVersion string                 `json:"installedVersion"`
	Updatable        bool                   `json:"updatable"`
	Reason           string                 `json:"reason,omitempty"`
	Latest           *LatestReleaseResponse `json:"latest"`
	CheckedAt        *time.Time             `json:"checkedAt"`
	State            string                 `json:"state"`
	TargetVersion    string                 `json:"targetVersion,omitempty"`
	Step             string                 `json:"step,omitempty"`
	StartedAt        *time.Time             `json:"startedAt,omitempty"`
	FinishedAt       *time.Time             `json:"finishedAt,omitempty"`
	Failure          *UpdateFailureResponse `json:"failure,omitempty"`
}

type LatestReleaseResponse struct {
	Version     string    `json:"version"`
	NotesURL    string    `json:"notesUrl"`
	PublishedAt time.Time `json:"publishedAt"`
}

type UpdateFailureResponse struct {
	Code       string `json:"code"`
	Step       string `json:"step"`
	RolledBack bool   `json:"rolledBack"`
}

func newUpdateStatusResponse(status UpdaterStatus) UpdateStatusResponse {
	response := UpdateStatusResponse{
		InstalledVersion: status.InstalledVersion, Updatable: status.Updatable, Reason: status.Reason, CheckedAt: status.CheckedAt,
		State: status.State, TargetVersion: status.TargetVersion, Step: status.Step, StartedAt: status.StartedAt, FinishedAt: status.FinishedAt,
	}
	if status.Latest != nil {
		response.Latest = &LatestReleaseResponse{Version: status.Latest.Version, NotesURL: status.Latest.NotesURL, PublishedAt: status.Latest.PublishedAt}
	}
	if status.Failure != nil {
		response.Failure = &UpdateFailureResponse{Code: status.Failure.Code, Step: status.Failure.Step, RolledBack: status.Failure.RolledBack}
	}
	return response
}

type OutdatedImageResponse struct {
	Reference string `json:"reference"`
	Size      int64  `json:"size"`
}

type CleanupPreviewResponse struct {
	Images     []OutdatedImageResponse `json:"images"`
	TotalBytes int64                   `json:"totalBytes"`
}

type CleanupReportResponse struct {
	Removed    []OutdatedImageResponse `json:"removed"`
	Skipped    []OutdatedImageResponse `json:"skipped"`
	FreedBytes int64                   `json:"freedBytes"`
}

func newOutdatedImageResponses(images []OutdatedImage) []OutdatedImageResponse {
	responses := make([]OutdatedImageResponse, 0, len(images))
	for _, image := range images {
		responses = append(responses, OutdatedImageResponse{Reference: image.Reference, Size: image.Size})
	}
	return responses
}

func newCleanupPreviewResponse(preview CleanupPreview) CleanupPreviewResponse {
	return CleanupPreviewResponse{Images: newOutdatedImageResponses(preview.Images), TotalBytes: preview.TotalBytes}
}

func newCleanupReportResponse(report CleanupReport) CleanupReportResponse {
	return CleanupReportResponse{Removed: newOutdatedImageResponses(report.Removed), Skipped: newOutdatedImageResponses(report.Skipped), FreedBytes: report.FreedBytes}
}
