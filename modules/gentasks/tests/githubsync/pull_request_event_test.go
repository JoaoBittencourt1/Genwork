package githubsync_test

import (
	"errors"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

func TestPullRequestStateFoldsGithubFields(t *testing.T) {
	cases := []struct {
		githubState   string
		draft, merged bool
		want          githubsync.PullRequestState
	}{
		{"open", false, false, githubsync.PullRequestOpen},
		{"open", true, false, githubsync.PullRequestDraft},
		{"closed", false, true, githubsync.PullRequestMerged},
		{"closed", false, false, githubsync.PullRequestClosed},
		{"closed", true, false, githubsync.PullRequestClosed},
	}
	for _, tc := range cases {
		if got := githubsync.PullRequestStateOf(tc.githubState, tc.draft, tc.merged); got != tc.want {
			t.Errorf("state(%q, draft %v, merged %v) = %q, want %q", tc.githubState, tc.draft, tc.merged, got, tc.want)
		}
	}
}

func TestDecodePullRequestEventReadsTheSnapshotFields(t *testing.T) {
	event, err := githubsync.DecodePullRequestEvent(githubsynctest.WebhookPayload(t, githubsynctest.PullRequestConvertedToDraft))

	if err != nil {
		t.Fatal(err)
	}
	snapshot := event.Snapshot()
	want := githubsync.PullRequestSnapshot{
		Repository: "acme/api", Number: 12, Title: "GEN-1: sign in with e-mail", State: githubsync.PullRequestDraft,
		Author:          githubsync.PullRequestAuthor{Login: githubsynctest.PullRequestAuthorLogin, AvatarURL: githubsynctest.PullRequestAuthorAvatarURL},
		GithubUpdatedAt: time.Date(2026, 10, 21, 12, 20, 0, 0, time.UTC),
	}
	if event.Action != githubsync.ActionConvertedToDraft || snapshot != want {
		t.Errorf("action = %q, snapshot = %+v", event.Action, snapshot)
	}
	if _, moves := event.Trigger(); moves {
		t.Error("converting to draft must not move tasks")
	}
}

func TestDecodePullRequestEventRequiresUpdatedAt(t *testing.T) {
	body := []byte(`{"action":"opened","pull_request":{"number":12,"title":"GEN-1"},"repository":{"full_name":"acme/api"}}`)

	if _, err := githubsync.DecodePullRequestEvent(body); !errors.Is(err, httpapi.ErrMalformedBody) {
		t.Errorf("err = %v", err)
	}
}

func TestPullRequestActionsHandled(t *testing.T) {
	for _, action := range []githubsync.PullRequestAction{
		githubsync.ActionOpened, githubsync.ActionReopened, githubsync.ActionEdited, githubsync.ActionReadyForReview,
		githubsync.ActionConvertedToDraft, githubsync.ActionReviewRequested, githubsync.ActionClosed,
	} {
		if !action.IsHandled() {
			t.Errorf("%q is not handled", action)
		}
	}
	for _, action := range []githubsync.PullRequestAction{"labeled", "synchronize", "assigned", ""} {
		if action.IsHandled() {
			t.Errorf("%q is handled", action)
		}
	}
}
