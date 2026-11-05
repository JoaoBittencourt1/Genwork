package taskcontext

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type TaskContextBoardResponse struct {
	Token      string `json:"token"`
	Name       string `json:"name"`
	TaskPrefix string `json:"taskPrefix"`
}

type TaskContextResponse struct {
	Token         string                               `json:"token"`
	Key           string                               `json:"key"`
	Title         string                               `json:"title"`
	Description   *string                              `json:"description"`
	AssigneeToken *string                              `json:"assigneeToken"`
	AssigneeKind  *tasks.AssigneeKind                  `json:"assigneeKind"`
	Labels        []labels.LabelResponse               `json:"labels"`
	Board         TaskContextBoardResponse             `json:"board"`
	PullRequests  []githubsync.TaskPullRequestResponse `json:"pullRequests"`
	Repository    *string                              `json:"repository"`
}

func NewTaskContextResponse(taskContext TaskContext, githubWebURL string) TaskContextResponse {
	task := taskContext.Task
	var assigneeKind *tasks.AssigneeKind
	if task.AssigneeUserToken != nil {
		assigneeKind = &task.AssigneeKind
	}
	return TaskContextResponse{
		Token: task.Token, Key: taskContext.Key(), Title: task.Title, Description: task.Description,
		AssigneeToken: task.AssigneeUserToken, AssigneeKind: assigneeKind,
		Labels:       labels.NewLabelResponses(taskContext.Labels),
		Board:        TaskContextBoardResponse{Token: taskContext.Board.Token, Name: taskContext.Board.Name, TaskPrefix: taskContext.Board.TaskPrefix},
		PullRequests: githubsync.NewTaskPullRequestResponses(taskContext.PullRequests, githubWebURL),
		Repository:   taskContext.Repository,
	}
}

type TaskContextHandler struct {
	service      *TaskContextService
	githubWebURL string
}

func NewTaskContextHandler(service *TaskContextService, githubWebURL string) *TaskContextHandler {
	return &TaskContextHandler{service: service, githubWebURL: githubWebURL}
}

func (handler *TaskContextHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/tasks/{token}/context", httpapi.RequirePermission(access.PermissionBoardRead), handler.findContext)
}

func (handler *TaskContextHandler) findContext(w http.ResponseWriter, r *http.Request) {
	taskContext, err := handler.service.FindContext(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewTaskContextResponse(taskContext, handler.githubWebURL))
}
