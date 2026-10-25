package boardview

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

// BoardTaskResponse is a task as the board shows it: the task endpoints' shape plus its linked pull requests.
type BoardTaskResponse struct {
	tasks.TaskResponse
	PullRequests []githubsync.TaskPullRequestResponse `json:"pullRequests"`
}

type ColumnWithTasksResponse struct {
	boards.ColumnResponse
	Tasks []BoardTaskResponse `json:"tasks"`
}

type BoardViewResponse struct {
	boards.BoardResponse
	Columns []ColumnWithTasksResponse `json:"columns"`
	Labels  []labels.LabelResponse    `json:"labels"`
}

func NewBoardViewResponse(view BoardView, githubWebURL string) BoardViewResponse {
	tasksByColumn := map[int64][]BoardTaskResponse{}
	for _, task := range view.Tasks {
		tasksByColumn[task.ColumnID] = append(tasksByColumn[task.ColumnID], BoardTaskResponse{
			TaskResponse: tasks.NewTaskResponse(task),
			PullRequests: githubsync.NewTaskPullRequestResponses(view.PullRequests[task.ID], githubWebURL),
		})
	}
	response := BoardViewResponse{
		BoardResponse: boards.NewBoardResponse(view.Board),
		Columns:       make([]ColumnWithTasksResponse, 0, len(view.Columns)),
		Labels:        labels.NewLabelResponses(view.Labels),
	}
	for _, column := range view.Columns {
		columnTasks := tasksByColumn[column.ID]
		if columnTasks == nil {
			columnTasks = []BoardTaskResponse{}
		}
		response.Columns = append(response.Columns, ColumnWithTasksResponse{ColumnResponse: boards.NewColumnResponse(column), Tasks: columnTasks})
	}
	return response
}

// BoardViewHandler.githubWebURL builds the links to pull requests (rule 1).
type BoardViewHandler struct {
	service      *BoardViewService
	githubWebURL string
}

func NewBoardViewHandler(service *BoardViewService, githubWebURL string) *BoardViewHandler {
	return &BoardViewHandler{service: service, githubWebURL: githubWebURL}
}

func (handler *BoardViewHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/boards/{token}", httpapi.RequirePermission(access.PermissionBoardRead), handler.readBoard)
}

func (handler *BoardViewHandler) readBoard(w http.ResponseWriter, r *http.Request) {
	view, err := handler.service.ReadBoard(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewBoardViewResponse(view, handler.githubWebURL))
}
