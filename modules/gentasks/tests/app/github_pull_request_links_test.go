package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
)

func (app *testApp) storedPullRequest(number int) (githubsync.GithubPullRequest, bool) {
	app.t.Helper()
	pullRequest, found, err := githubsync.GithubPullRequestRepository{}.FindByNumber(context.Background(), app.db, "acme/api", number)
	if err != nil {
		app.t.Fatal(err)
	}
	return pullRequest, found
}

func (app *testApp) expectPullRequestState(number int, want githubsync.PullRequestState) {
	app.t.Helper()
	if pullRequest, found := app.storedPullRequest(number); !found || pullRequest.State == nil || *pullRequest.State != want {
		app.t.Errorf("pull request #%d = %+v, want state %q", number, pullRequest, want)
	}
}

func deliveredAt(minutes int) time.Time {
	return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

func TestWebhookStoresThePullRequestSnapshot(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.todo, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})

	links := app.pullRequestLinks(task.Token)
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	pullRequest := links[0].PullRequest
	if *pullRequest.Title != "GEN-1: sign in with e-mail" || *pullRequest.State != githubsync.PullRequestOpen ||
		pullRequest.Author.Login != githubsynctest.PullRequestAuthorLogin || pullRequest.Author.AvatarURL != githubsynctest.PullRequestAuthorAvatarURL {
		t.Errorf("pull request = %+v", pullRequest)
	}

	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{Title: "GEN-1: sign in with a magic link"})
	if updated, _ := app.storedPullRequest(12); *updated.Title != "GEN-1: sign in with a magic link" {
		t.Errorf("title = %q", *updated.Title)
	}
}

func TestWebhookLinksTasksItDoesNotMove(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.inReview, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.inReview, "Sign in")
	if links := app.pullRequestLinks(task.Token); len(links) != 1 || links[0].Link.LastMovedEvent != nil {
		t.Errorf("links = %+v", links)
	}
}

func TestWebhookTracksDraftAndClosedPullRequestsWithoutMoving(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.todo, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestConvertedToDraft, githubsynctest.PullRequestChanges{})
	app.expectPullRequestState(12, githubsync.PullRequestDraft)
	app.deliverPullRequest(githubsynctest.PullRequestClosedUnmerged, githubsynctest.PullRequestChanges{})
	app.expectPullRequestState(12, githubsync.PullRequestClosed)

	app.expectTitles(board.token, board.todo, "Sign in")
	if links := app.pullRequestLinks(task.Token); len(links) != 1 {
		t.Errorf("links = %+v", links)
	}
	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{})
	app.expectPullRequestState(12, githubsync.PullRequestMerged)
}

func TestWebhookUnlinksTasksThePullRequestNoLongerCites(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	dropped := app.createTaskIn(board.token, board.todo, "Sign in")
	kept := app.createTaskIn(board.token, board.todo, "Profile")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Title: "GEN-1 and GEN-2", HeadRef: "feat/account"})
	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{Title: "GEN-2 only", HeadRef: "feat/account"})

	if links := app.pullRequestLinks(dropped.Token); len(links) != 0 {
		t.Errorf("dropped links = %+v", links)
	}
	if links := app.pullRequestLinks(kept.Token); len(links) != 1 {
		t.Errorf("kept links = %+v", links)
	}

	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{
		Title: "GEN-1 and GEN-2 again", HeadRef: "feat/account", UpdatedAt: deliveredAt(6),
	})
	if links := app.pullRequestLinks(dropped.Token); len(links) != 1 {
		t.Errorf("relinked = %+v", links)
	}
}

func TestWebhookKeepsTheNewestSnapshotWhenDeliveriesArriveOutOfOrder(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.todo, "Sign in")
	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{Title: "GEN-1 newest", UpdatedAt: deliveredAt(5)})

	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{
		Title: "Older title without the key", HeadRef: "chore/older", UpdatedAt: deliveredAt(1),
	})

	if pullRequest, _ := app.storedPullRequest(12); *pullRequest.Title != "GEN-1 newest" {
		t.Errorf("title = %q", *pullRequest.Title)
	}
	if links := app.pullRequestLinks(task.Token); len(links) != 1 {
		t.Errorf("links = %+v", links)
	}
}

func TestWebhookStoresNothingForPullRequestsThatCiteNoTask(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.todo, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Title: "Refactor", HeadRef: "chore/refactor"})
	app.deliverPullRequest(githubsynctest.PullRequestLabeled, githubsynctest.PullRequestChanges{Number: 40})

	for _, number := range []int{12, 40} {
		if _, found := app.storedPullRequest(number); found {
			t.Errorf("pull request #%d was stored", number)
		}
	}
}

func TestWebhookUnlinksEveryTaskWhenThePullRequestStopsCitingTasks(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.todo, "Sign in")
	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})

	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{Title: "Refactor", HeadRef: "chore/refactor"})

	if links := app.pullRequestLinks(task.Token); len(links) != 0 {
		t.Errorf("links = %+v", links)
	}
	if pullRequest, _ := app.storedPullRequest(12); *pullRequest.Title != "Refactor" {
		t.Errorf("title = %q", *pullRequest.Title)
	}
}

func (app *testApp) boardTask(boardToken, taskToken string) taskBody {
	app.t.Helper()
	for _, column := range app.readBoard(boardToken).Columns {
		for _, task := range column.Tasks {
			if task.Token == taskToken {
				return task
			}
		}
	}
	app.t.Fatalf("task %s is not on board %s", taskToken, boardToken)
	return taskBody{}
}

func TestBoardViewListsEachTasksPullRequests(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	linked := app.createTaskIn(board.token, board.todo, "Sign in")
	unlinked := app.createTaskIn(board.token, board.todo, "Profile")
	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})
	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{Number: 13, Title: "GEN-1 follow-up", HeadRef: "fix/follow-up"})

	pullRequests := app.boardTask(board.token, linked.Token).PullRequests

	if len(pullRequests) != 2 {
		t.Fatalf("pull requests = %+v", pullRequests)
	}
	open := pullRequests[0]
	if len(open.Token) != 22 || open.Repository != "acme/api" || open.Number != 12 || *open.Title != "GEN-1: sign in with e-mail" ||
		*open.State != "open" || open.Author == nil || open.Author.Login != githubsynctest.PullRequestAuthorLogin ||
		open.Author.AvatarURL != githubsynctest.PullRequestAuthorAvatarURL || open.URL != githubsynctest.UnroutableGithubWebURL+"/acme/api/pull/12" {
		t.Errorf("open pull request = %+v", open)
	}
	if merged := pullRequests[1]; merged.Number != 13 || *merged.State != "merged" || *merged.Title != "GEN-1 follow-up" {
		t.Errorf("merged pull request = %+v", merged)
	}
	if others := app.boardTask(board.token, unlinked.Token).PullRequests; others == nil || len(others) != 0 {
		t.Errorf("pull requests of an unlinked task = %#v, want []", others)
	}
}

func TestBoardViewOmitsUnlinkedPullRequests(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.todo, "Sign in")
	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})

	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{Title: "Refactor", HeadRef: "chore/refactor"})

	if pullRequests := app.boardTask(board.token, task.Token).PullRequests; len(pullRequests) != 0 {
		t.Errorf("pull requests = %+v", pullRequests)
	}
}

func TestBoardViewShowsPullRequestsLinkedBeforeSnapshotsWithGithubDisconnected(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoardWithPrefix("Platform", "GEN")
	task := app.createTaskIn(board.Token, board.Columns[1].Token, "Sign in")
	ctx := context.Background()
	var pullRequestID int64
	err := app.db.QueryRow(ctx, `
		INSERT INTO github_pull_requests (token, repository, number, created_at, updated_at)
		VALUES ('migrated-pull-request', 'acme/api', 3, now(), now()) RETURNING id`).Scan(&pullRequestID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.db.Exec(ctx, `
		INSERT INTO task_pull_requests (token, task_id, pull_request_id, last_moved_event, moved_at, merge_moved_at, created_at, updated_at)
		SELECT 'migrated-link', id, $1, 'merged', now(), now(), now(), now() FROM tasks WHERE token = $2`, pullRequestID, task.Token)
	if err != nil {
		t.Fatal(err)
	}

	pullRequests := app.boardTask(board.Token, task.Token).PullRequests

	if len(pullRequests) != 1 {
		t.Fatalf("pull requests = %+v", pullRequests)
	}
	migrated := pullRequests[0]
	if migrated.Number != 3 || migrated.Title != nil || migrated.State != nil || migrated.Author != nil ||
		migrated.URL != githubsynctest.UnroutableGithubWebURL+"/acme/api/pull/3" {
		t.Errorf("migrated pull request = %+v", migrated)
	}
}

func TestTaskEndpointsKeepTheirShape(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Platform")
	task := app.createTask(board.Token, map[string]any{"title": "Sign in"})

	for _, path := range []string{"/api/gentasks/v1/tasks/" + task.Token, "/api/gentasks/v1/boards/" + board.Token + "/tasks"} {
		resp := app.do(request{method: "GET", path: path})
		expectStatus(t, resp, http.StatusOK)
		if strings.Contains(string(resp.body), "pullRequests") {
			t.Errorf("%s returns pull requests: %s", path, resp.body)
		}
	}
}
