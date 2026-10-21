package boardview

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/boards"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/tasks"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type ColumnWithTasksResponse struct {
	boards.ColumnResponse
	Tasks []tasks.TaskResponse `json:"tasks"`
}

type BoardViewResponse struct {
	boards.BoardResponse
	Columns []ColumnWithTasksResponse `json:"columns"`
	Labels  []labels.LabelResponse    `json:"labels"`
}

func NewBoardViewResponse(view BoardView) BoardViewResponse {
	tasksByColumn := map[int64][]tasks.TaskResponse{}
	for _, task := range view.Tasks {
		tasksByColumn[task.ColumnID] = append(tasksByColumn[task.ColumnID], tasks.NewTaskResponse(task))
	}
	response := BoardViewResponse{
		BoardResponse: boards.NewBoardResponse(view.Board),
		Columns:       make([]ColumnWithTasksResponse, 0, len(view.Columns)),
		Labels:        labels.NewLabelResponses(view.Labels),
	}
	for _, column := range view.Columns {
		columnTasks := tasksByColumn[column.ID]
		if columnTasks == nil {
			columnTasks = []tasks.TaskResponse{}
		}
		response.Columns = append(response.Columns, ColumnWithTasksResponse{ColumnResponse: boards.NewColumnResponse(column), Tasks: columnTasks})
	}
	return response
}

type BoardViewHandler struct {
	service *BoardViewService
}

func NewBoardViewHandler(service *BoardViewService) *BoardViewHandler {
	return &BoardViewHandler{service: service}
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
	httpapi.WriteJSON(w, http.StatusOK, NewBoardViewResponse(view))
}
