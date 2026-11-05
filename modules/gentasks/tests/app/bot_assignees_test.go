package app_test

import (
	"net/http"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coreclient"
)

const botToken = "dev-bot-token"

type taskContextBody struct {
	Token         string  `json:"token"`
	Key           string  `json:"key"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	AssigneeToken *string `json:"assigneeToken"`
	AssigneeKind  *string `json:"assigneeKind"`
	Labels        []labelBody
	Board         struct {
		Token      string `json:"token"`
		Name       string `json:"name"`
		TaskPrefix string `json:"taskPrefix"`
	} `json:"board"`
	PullRequests []pullRequestBody `json:"pullRequests"`
	Repository   *string           `json:"repository"`
}

func (app *testApp) addBotMember() {
	app.core.AddUser(coreclient.DirectoryUser{Token: botToken, Kind: "bot", Name: "Dev Bot"})
}

func (app *testApp) readTaskContext(taskToken string) taskContextBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/" + taskToken + "/context"})
	expectStatus(app.t, resp, http.StatusOK)
	var body taskContextBody
	resp.decode(app.t, &body)
	return body
}

func TestBotMemberIsAcceptedAsAssignee(t *testing.T) {
	app := newTestApp(t)
	app.addBotMember()
	board := app.createBoard("Genwork")

	created := app.createTask(board.Token, map[string]any{"title": "Fix it", "assigneeToken": botToken})

	if created.AssigneeToken == nil || *created.AssigneeToken != botToken || created.AssigneeKind == nil || *created.AssigneeKind != "bot" {
		t.Fatalf("created task = %+v", created)
	}
	fromBoard := app.readBoard(board.Token)
	if got := fromBoard.Columns[0].Tasks; len(got) != 1 || got[0].AssigneeKind == nil || *got[0].AssigneeKind != "bot" {
		t.Errorf("board view tasks = %+v", got)
	}
}

func TestAssigneeKindFollowsAssigneeChanges(t *testing.T) {
	app := newTestApp(t)
	app.addBotMember()
	board := app.createBoard("Genwork")
	task := app.createTask(board.Token, map[string]any{"title": "Fix it", "assigneeToken": userToken})
	if task.AssigneeKind == nil || *task.AssigneeKind != "human" {
		t.Fatalf("human assignee kind = %v", task.AssigneeKind)
	}

	toBot := app.patchTask(task.Token, map[string]any{"assigneeToken": botToken})
	cleared := app.patchTask(task.Token, map[string]any{"assigneeToken": nil})

	if toBot.AssigneeKind == nil || *toBot.AssigneeKind != "bot" {
		t.Errorf("bot assignee kind = %v", toBot.AssigneeKind)
	}
	if cleared.AssigneeToken != nil || cleared.AssigneeKind != nil {
		t.Errorf("cleared task = %+v", cleared)
	}
	if found := app.findTask(task.Token); found.AssigneeKind != nil {
		t.Errorf("found task kind = %v", found.AssigneeKind)
	}
}

func TestPatchingTitleKeepsTheAssigneeKind(t *testing.T) {
	app := newTestApp(t)
	app.addBotMember()
	board := app.createBoard("Genwork")
	task := app.createTask(board.Token, map[string]any{"title": "Fix it", "assigneeToken": botToken})

	renamed := app.patchTask(task.Token, map[string]any{"title": "Fix it better"})

	if renamed.AssigneeKind == nil || *renamed.AssigneeKind != "bot" || renamed.AssigneeToken == nil || *renamed.AssigneeToken != botToken {
		t.Errorf("renamed task = %+v", renamed)
	}
}

func TestBoardRepositorySetting(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Genwork")
	if repository := app.readBoardGithubSettings(board.Token).Repository; repository != nil {
		t.Fatalf("default repository = %v", *repository)
	}
	column := app.addColumn(board.Token, "Review")
	before := app.patchBoardGithubSettings(board.Token, map[string]any{"openedColumnToken": column.Token, "finalBranches": []string{"develop"}})

	after := app.patchBoardGithubSettings(board.Token, map[string]any{"repository": "acme/api"})

	if after.Repository == nil || *after.Repository != "acme/api" {
		t.Fatalf("repository = %v", after.Repository)
	}
	if after.OpenedColumnToken == nil || *after.OpenedColumnToken != *before.OpenedColumnToken ||
		len(after.FinalBranches) != 1 || after.FinalBranches[0] != "develop" {
		t.Errorf("a single-field PATCH changed other settings: before %+v, after %+v", before, after)
	}
	cleared := app.patchBoardGithubSettings(board.Token, map[string]any{"repository": nil})
	if cleared.Repository != nil || cleared.OpenedColumnToken == nil {
		t.Errorf("cleared settings = %+v", cleared)
	}
}

func TestBoardRepositoryMustBeOwnerAndName(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Genwork")

	for _, repository := range []string{"acme", "acme/", "/api", "a b/c", "a/b/c", ""} {
		resp := app.do(request{method: "PATCH", path: githubSettingsPath(board.Token), body: map[string]any{"repository": repository}})
		expectStatus(t, resp, http.StatusBadRequest)
	}
}

func TestBoardRepositoryNeedsBoardManage(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Genwork")

	resp := app.do(request{method: "PATCH", path: githubSettingsPath(board.Token), body: map[string]any{"repository": "acme/api"}, accessToken: app.tokenWith(access.PermissionBoardRead)})

	expectStatus(t, resp, http.StatusForbidden)
}

func TestTaskContextForBots(t *testing.T) {
	app := newTestApp(t)
	app.addBotMember()
	board := app.createBoard("Genwork")
	app.patchBoardGithubSettings(board.Token, map[string]any{"repository": "acme/api"})
	label := app.createLabel(board.Token, "backend", "blue")
	description := "Make the API faster"
	task := app.createTask(board.Token, map[string]any{
		"title": "Speed up", "description": description, "assigneeToken": botToken, "labelTokens": []string{label.Token},
	})

	context := app.readTaskContext(task.Token)

	if context.Key != board.TaskPrefix+"-1" || context.Title != "Speed up" || context.Description == nil || *context.Description != description ||
		context.Board.Name != "Genwork" || context.Repository == nil || *context.Repository != "acme/api" ||
		len(context.Labels) != 1 || context.PullRequests == nil || len(context.PullRequests) != 0 ||
		context.AssigneeKind == nil || *context.AssigneeKind != "bot" {
		t.Errorf("context = %+v", context)
	}
}

func TestTaskContextRules(t *testing.T) {
	app := newTestApp(t)
	board := app.createBoard("Genwork")
	task := app.createTask(board.Token, map[string]any{"title": "Speed up"})

	forbidden := app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/" + task.Token + "/context", accessToken: app.tokenWith()})
	missing := app.do(request{method: "GET", path: "/api/gentasks/v1/tasks/unknown/context"})

	expectStatus(t, forbidden, http.StatusForbidden)
	expectStatus(t, missing, http.StatusNotFound)
	if context := app.readTaskContext(task.Token); context.Repository != nil || context.AssigneeKind != nil {
		t.Errorf("context = %+v", context)
	}
}
