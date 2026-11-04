package app_test

import (
	"net/http"
	"testing"
	"time"
)

type boardBody struct {
	Token      string    `json:"token"`
	Name       string    `json:"name"`
	TaskPrefix string    `json:"taskPrefix"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type labelBody struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type taskBody struct {
	Token         string      `json:"token"`
	Number        int         `json:"number"`
	ColumnToken   string      `json:"columnToken"`
	Position      *int        `json:"position"`
	Title         string      `json:"title"`
	Description   *string     `json:"description"`
	AssigneeToken *string     `json:"assigneeToken"`
	Urgency       string      `json:"urgency"`
	Labels        []labelBody `json:"labels"`
	CreatedBy     string      `json:"createdBy"`
	AssignedAt    *time.Time  `json:"assignedAt"`
	ArchivedAt    *time.Time  `json:"archivedAt"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	// PullRequests is only part of the board view's tasks.
	PullRequests []pullRequestBody `json:"pullRequests"`
}

type pullRequestBody struct {
	Token      string  `json:"token"`
	Repository string  `json:"repository"`
	Number     int     `json:"number"`
	Title      *string `json:"title"`
	State      *string `json:"state"`
	Author     *struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatarUrl"`
	} `json:"author"`
	URL string `json:"url"`
}

type columnBody struct {
	Token    string     `json:"token"`
	Preset   *string    `json:"preset"`
	Name     *string    `json:"name"`
	Position int        `json:"position"`
	Tasks    []taskBody `json:"tasks"`
}

type boardDetailBody struct {
	boardBody
	Columns []columnBody `json:"columns"`
	Labels  []labelBody  `json:"labels"`
}

func (app *testApp) createBoard(name string) boardDetailBody {
	app.t.Helper()
	resp := app.do(request{method: "POST", path: "/api/gentasks/v1/boards", body: map[string]any{"name": name}})
	expectStatus(app.t, resp, http.StatusCreated)
	var board boardDetailBody
	resp.decode(app.t, &board)
	return board
}

func (app *testApp) patchBoard(token string, body map[string]any) response {
	return app.do(request{method: "PATCH", path: "/api/gentasks/v1/boards/" + token, body: body})
}

func (app *testApp) readBoard(token string) boardDetailBody {
	app.t.Helper()
	resp := app.do(request{method: "GET", path: "/api/gentasks/v1/boards/" + token})
	expectStatus(app.t, resp, http.StatusOK)
	var board boardDetailBody
	resp.decode(app.t, &board)
	return board
}

func columnLabels(t *testing.T, columns []columnBody) []string {
	t.Helper()
	labels := make([]string, 0, len(columns))
	for position, column := range columns {
		if column.Position != position {
			t.Fatalf("column %s has position %d, want %d", column.Token, column.Position, position)
		}
		switch {
		case column.Name != nil:
			labels = append(labels, *column.Name)
		case column.Preset != nil:
			labels = append(labels, *column.Preset)
		}
	}
	return labels
}
