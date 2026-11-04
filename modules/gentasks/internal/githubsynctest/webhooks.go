package githubsynctest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

// Recorded webhook deliveries in testdata/webhooks; the pull request ones cite GEN-1 in the title and branch, base main.
const (
	PullRequestOpened           = "pull_request_opened.json"
	PullRequestReopened         = "pull_request_reopened.json"
	PullRequestEdited           = "pull_request_edited.json"
	PullRequestReadyForReview   = "pull_request_ready_for_review.json"
	PullRequestConvertedToDraft = "pull_request_converted_to_draft.json"
	PullRequestReviewRequested  = "pull_request_review_requested.json"
	PullRequestClosedMerged     = "pull_request_closed_merged.json"
	PullRequestClosedUnmerged   = "pull_request_closed_unmerged.json"
	PullRequestLabeled          = "pull_request_labeled.json"
	Ping                        = "ping.json"
	InstallationCreated         = "installation_created.json"
)

// PullRequestChanges rewrites a recorded pull request; empty fields keep the recorded value.
type PullRequestChanges struct {
	Title     string
	HeadRef   string
	BaseRef   string
	Body      string
	Number    int
	UpdatedAt time.Time
}

func WebhookPayload(t testing.TB, name string) []byte {
	t.Helper()
	body, err := recordedResponses.ReadFile("testdata/webhooks/" + name)
	if err != nil {
		t.Fatalf("read webhook fixture %s: %v", name, err)
	}
	return body
}

func PullRequestPayload(t testing.TB, name string, changes PullRequestChanges) []byte {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(WebhookPayload(t, name), &payload); err != nil {
		t.Fatalf("decode webhook fixture %s: %v", name, err)
	}
	pullRequest := payload["pull_request"].(map[string]any)
	setIfChanged(pullRequest, "title", changes.Title)
	setIfChanged(pullRequest, "body", changes.Body)
	setIfChanged(pullRequest["head"].(map[string]any), "ref", changes.HeadRef)
	setIfChanged(pullRequest["base"].(map[string]any), "ref", changes.BaseRef)
	if !changes.UpdatedAt.IsZero() {
		pullRequest["updated_at"] = changes.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if changes.Number != 0 {
		pullRequest["number"], payload["number"] = changes.Number, changes.Number
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode webhook fixture %s: %v", name, err)
	}
	return body
}

// SignWebhookPayload computes the X-Hub-Signature-256 header GitHub sends with a delivery.
func SignWebhookPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func setIfChanged(object map[string]any, key, value string) {
	if value != "" {
		object[key] = value
	}
}
