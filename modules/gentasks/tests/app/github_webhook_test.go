package app_test

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
)

const webhookPath = "/api/gentasks/v1/github/webhook"

// syncBoard is a board whose columns read backlog, todo, in progress, in review, done, with GitHub settings mapping
// PR opened to in progress, review requested to in review, and merges to done.
type syncBoard struct {
	token      string
	todo       string
	inProgress string
	inReview   string
	done       string
}

func (app *testApp) createBoardWithPrefix(name, prefix string) boardDetailBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards", body: map[string]any{"name": name, "taskPrefix": prefix}})
	expectStatus(app.t, resp, http.StatusCreated)
	var board boardDetailBody
	resp.decode(app.t, &board)
	return board
}

func (app *testApp) createSyncBoard(prefix string) syncBoard {
	app.t.Helper()
	board := app.createBoardWithPrefix(prefix+" board", prefix)
	inProgress := app.addColumn(board.Token, "In progress")
	inReview := app.addColumn(board.Token, "In review")
	for position, column := range []string{inProgress.Token, inReview.Token} {
		expectStatus(app.t, app.do(request{method: "POST", path: "/api/gentasks/v1/columns/" + column + "/move", body: map[string]any{"position": 2 + position}}), http.StatusNoContent)
	}
	app.patchBoardGithubSettings(board.Token, map[string]any{"openedColumnToken": inProgress.Token, "reviewRequestedColumnToken": inReview.Token})
	return syncBoard{token: board.Token, todo: board.Columns[1].Token, inProgress: inProgress.Token, inReview: inReview.Token, done: board.Columns[2].Token}
}

func (app *testApp) createTaskIn(boardToken, columnToken, title string) taskBody {
	app.t.Helper()
	return app.createTask(boardToken, map[string]any{"title": title, "columnToken": columnToken})
}

func (app *testApp) deliverGithubEvent(event string, body []byte) response {
	app.t.Helper()
	return app.deliverSignedGithubEvent(event, body, githubsynctest.SignWebhookPayload(githubsynctest.WebhookSecret, body))
}

func (app *testApp) deliverSignedGithubEvent(event string, body []byte, signature string) response {
	app.t.Helper()
	headers := map[string]string{"X-GitHub-Event": event, "X-GitHub-Delivery": "72d3162e-cc78-11e3-81ab-4c9367dc0958"}
	if signature != "" {
		headers["X-Hub-Signature-256"] = signature
	}
	return app.do(request{method: "POST", path: webhookPath, rawBytes: body, anonymous: true, headers: headers})
}

func (app *testApp) deliverPullRequest(fixture string, changes githubsynctest.PullRequestChanges) {
	app.t.Helper()
	expectStatus(app.t, app.deliverGithubEvent("pull_request", githubsynctest.PullRequestPayload(app.t, fixture, changes)), http.StatusNoContent)
}

func (app *testApp) expectTitles(boardToken, columnToken string, want ...string) {
	app.t.Helper()
	if got := app.titlesOf(boardToken, columnToken); !slices.Equal(got, want) {
		app.t.Errorf("column titles = %v, want %v", got, want)
	}
}

func (app *testApp) pullRequestLinks(taskToken string) []githubsync.LinkedPullRequest {
	app.t.Helper()
	task, err := tasks.TaskRepository{}.FindByToken(context.Background(), app.db, taskToken)
	if err != nil {
		app.t.Fatal(err)
	}
	linked, err := githubsync.TaskPullRequestRepository{}.ListActiveForTasks(context.Background(), app.db, []int64{task.ID})
	if err != nil {
		app.t.Fatal(err)
	}
	return linked[task.ID]
}

func movedBy(link githubsync.LinkedPullRequest, trigger githubsync.PullRequestTrigger) bool {
	return link.Link.LastMovedEvent != nil && *link.Link.LastMovedEvent == trigger
}

func newConnectedTestApp(t *testing.T) *testApp {
	app := newTestApp(t)
	githubsynctest.StoreGithubApp(t, app.db, githubsynctest.EncryptionKey)
	return app
}

func TestWebhookRejectsInvalidSignatures(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.todo, "Sign in")
	body := githubsynctest.WebhookPayload(t, githubsynctest.PullRequestOpened)

	for _, signature := range []string{"", "sha256=00", githubsynctest.SignWebhookPayload("another-secret", body)} {
		expectError(t, app.deliverSignedGithubEvent("pull_request", body, signature), http.StatusUnauthorized, "invalid_signature")
	}
	app.expectTitles(board.token, board.todo, "Sign in")
}

func TestWebhookRejectsDeliveriesWithoutReadableApp(t *testing.T) {
	disconnected := newTestApp(t)
	body := githubsynctest.WebhookPayload(t, githubsynctest.PullRequestOpened)
	expectError(t, disconnected.deliverGithubEvent("pull_request", body), http.StatusUnauthorized, "invalid_signature")

	unreadable := newTestApp(t)
	githubsynctest.StoreGithubApp(t, unreadable.db, githubsynctest.OtherEncryptionKey)
	expectError(t, unreadable.deliverGithubEvent("pull_request", body), http.StatusUnauthorized, "invalid_signature")
}

func TestWebhookRejectsOversizedBodies(t *testing.T) {
	app := newConnectedTestApp(t)
	body := bytes.Repeat([]byte(" "), 5<<20+1)

	expectError(t, app.deliverGithubEvent("pull_request", body), http.StatusRequestEntityTooLarge, "payload_too_large")
}

func TestWebhookRejectsMalformedPullRequests(t *testing.T) {
	app := newConnectedTestApp(t)
	for _, body := range []string{`{"action":`, `{"action":"opened","pull_request":{"number":0},"repository":{"full_name":"acme/api"}}`} {
		expectError(t, app.deliverGithubEvent("pull_request", []byte(body)), http.StatusBadRequest, "malformed_body")
	}
}

func TestWebhookAcknowledgesIgnoredEvents(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.todo, "Sign in")

	expectStatus(t, app.deliverGithubEvent("ping", githubsynctest.WebhookPayload(t, githubsynctest.Ping)), http.StatusNoContent)
	expectStatus(t, app.deliverGithubEvent("installation", githubsynctest.WebhookPayload(t, githubsynctest.InstallationCreated)), http.StatusNoContent)
	app.deliverPullRequest(githubsynctest.PullRequestLabeled, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.todo, "Sign in")
}

func TestWebhookMovesTaskWhenPullRequestOpened(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.todo, "Sign in")
	app.createTaskIn(board.token, board.inProgress, "Already started")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.todo)
	app.expectTitles(board.token, board.inProgress, "Already started", "Sign in")
	links := app.pullRequestLinks(task.Token)
	if len(links) != 1 || links[0].PullRequest.Repository != "acme/api" || links[0].PullRequest.Number != 12 ||
		!movedBy(links[0], githubsync.TriggerOpened) || links[0].Link.MergeMovedAt != nil {
		t.Errorf("links = %+v", links)
	}
}

func TestWebhookTreatsReopenedEditedAndReadyForReviewAsOpened(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.todo, "Reopened")
	app.createTaskIn(board.token, board.todo, "Edited")
	app.createTaskIn(board.token, board.todo, "Ready")

	app.deliverPullRequest(githubsynctest.PullRequestReopened, githubsynctest.PullRequestChanges{Title: "GEN-1", HeadRef: "fix/reopened"})
	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{Title: "GEN-2", HeadRef: "fix/edited"})
	app.deliverPullRequest(githubsynctest.PullRequestReadyForReview, githubsynctest.PullRequestChanges{Title: "GEN-3", HeadRef: "fix/ready"})

	app.expectTitles(board.token, board.inProgress, "Reopened", "Edited", "Ready")
}

func TestWebhookMovesTaskWhenReviewIsRequested(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.inProgress, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestReviewRequested, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.inReview, "Sign in")
}

func TestWebhookMovesAndLocksTaskWhenMergedIntoAFinalBranch(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.inReview, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.done, "Sign in")
	if links := app.pullRequestLinks(task.Token); len(links) != 1 || links[0].Link.MergeMovedAt == nil || !movedBy(links[0], githubsync.TriggerMerged) {
		t.Errorf("links = %+v", links)
	}

	expectStatus(t, app.moveTask(task.Token, board.todo, 0), http.StatusNoContent)
	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Number: 13, HeadRef: "fix/gen-1-follow-up"})

	app.expectTitles(board.token, board.todo, "Sign in")
}

func TestWebhookIgnoresMergesOutsideFinalBranchesAndClosedPullRequests(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.inReview, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{BaseRef: "develop"})
	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{BaseRef: "Main"})
	app.deliverPullRequest(githubsynctest.PullRequestClosedUnmerged, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.inReview, "Sign in")
}

func TestWebhookMergesIntoConfiguredFinalBranches(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.patchBoardGithubSettings(board.token, map[string]any{"finalBranches": []string{"staging"}})
	app.createTaskIn(board.token, board.inReview, "Sign in")
	app.createTaskIn(board.token, board.inReview, "Profile")

	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{Title: "GEN-1", HeadRef: "a", BaseRef: "main"})
	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{Title: "GEN-2", HeadRef: "b", BaseRef: "staging"})

	app.expectTitles(board.token, board.inReview, "Sign in")
	app.expectTitles(board.token, board.done, "Profile")
}

func TestWebhookNeverMovesTasksBackwards(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.inReview, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestEdited, githubsynctest.PullRequestChanges{})

	app.expectTitles(board.token, board.inReview, "Sign in")
}

func TestWebhookIgnoresUnconfiguredEvents(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createBoardWithPrefix("Defaults", "GEN")
	app.createTaskIn(board.Token, board.Columns[1].Token, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{})
	app.deliverPullRequest(githubsynctest.PullRequestReviewRequested, githubsynctest.PullRequestChanges{})
	app.expectTitles(board.Token, board.Columns[1].Token, "Sign in")

	app.deliverPullRequest(githubsynctest.PullRequestClosedMerged, githubsynctest.PullRequestChanges{})
	app.expectTitles(board.Token, board.Columns[2].Token, "Sign in")
}

func TestWebhookRedeliveryChangesNothing(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	task := app.createTaskIn(board.token, board.inProgress, "Sign in")
	app.createTaskIn(board.token, board.inReview, "Other")
	body := githubsynctest.WebhookPayload(t, githubsynctest.PullRequestReviewRequested)

	for range 2 {
		expectStatus(t, app.deliverGithubEvent("pull_request", body), http.StatusNoContent)
	}

	app.expectTitles(board.token, board.inReview, "Other", "Sign in")
	if links := app.pullRequestLinks(task.Token); len(links) != 1 {
		t.Errorf("links = %+v", links)
	}
}

func TestWebhookMovesEveryCitedTaskByItsOwnBoard(t *testing.T) {
	app := newConnectedTestApp(t)
	gen := app.createSyncBoard("GEN")
	ops := app.createSyncBoard("OPS")
	app.patchBoardGithubSettings(ops.token, map[string]any{"openedColumnToken": ops.inReview})
	app.createTaskIn(gen.token, gen.todo, "Sign in")
	app.createTaskIn(ops.token, ops.todo, "Share login")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Title: "GEN-1 and ops-1: share login", HeadRef: "feat/share-login"})

	app.expectTitles(gen.token, gen.inProgress, "Sign in")
	app.expectTitles(ops.token, ops.inReview, "Share login")
}

func TestWebhookFindsKeysInTitleAndBranchOnly(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	app.createTaskIn(board.token, board.todo, "Sign in")
	app.createTaskIn(board.token, board.todo, "Profile")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Title: "Refactor", HeadRef: "chore/refactor", Body: "Prepares GEN-1"})
	app.expectTitles(board.token, board.todo, "Sign in", "Profile")

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Title: "Profile page", HeadRef: "feat/gen-2-profile"})
	app.expectTitles(board.token, board.inProgress, "Profile")
}

func TestWebhookIgnoresUnknownAndArchivedTasks(t *testing.T) {
	app := newConnectedTestApp(t)
	board := app.createSyncBoard("GEN")
	archived := app.createTaskIn(board.token, board.todo, "Archived")
	expectStatus(t, app.do(request{method: "POST", path: "/api/gentasks/v1/tasks/" + archived.Token + "/archive"}), http.StatusNoContent)

	app.deliverPullRequest(githubsynctest.PullRequestOpened, githubsynctest.PullRequestChanges{Title: "GEN-1 and GEN-999", HeadRef: "feat/x"})

	if found := app.findTask(archived.Token); found.ArchivedAt == nil || found.ColumnToken != board.todo {
		t.Errorf("archived task = %+v", found)
	}
	app.expectTitles(board.token, board.inProgress)
}
