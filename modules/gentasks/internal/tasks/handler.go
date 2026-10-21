package tasks

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/labels"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type TaskResponse struct {
	Token         string                 `json:"token"`
	ColumnToken   string                 `json:"columnToken"`
	Position      *int                   `json:"position"`
	Title         string                 `json:"title"`
	Description   *string                `json:"description"`
	AssigneeToken *string                `json:"assigneeToken"`
	Urgency       Urgency                `json:"urgency"`
	Labels        []labels.LabelResponse `json:"labels"`
	CreatedBy     string                 `json:"createdBy"`
	ArchivedAt    *time.Time             `json:"archivedAt"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
}

type TaskListResponse struct {
	Tasks []TaskResponse `json:"tasks"`
}

func NewTaskResponse(task TaskWithLabels) TaskResponse {
	return TaskResponse{
		Token:         task.Token,
		ColumnToken:   task.ColumnToken,
		Position:      task.Position,
		Title:         task.Title,
		Description:   task.Description,
		AssigneeToken: task.AssigneeUserToken,
		Urgency:       task.Urgency,
		Labels:        labels.NewLabelResponses(task.Labels),
		CreatedBy:     task.CreatedBy,
		ArchivedAt:    task.ArchivedAt,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
	}
}

func NewTaskResponses(listed []TaskWithLabels) []TaskResponse {
	responses := make([]TaskResponse, 0, len(listed))
	for _, task := range listed {
		responses = append(responses, NewTaskResponse(task))
	}
	return responses
}

type TaskHandler struct {
	service *TaskService
}

func NewTaskHandler(service *TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

func (handler *TaskHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/boards/{token}/tasks", httpapi.RequirePermission(access.PermissionBoardRead), handler.listTasks)
	router.HandleFunc("POST /api/gentasks/v1/boards/{token}/tasks", httpapi.RequirePermission(access.PermissionTaskWrite), handler.createTask)
	router.HandleFunc("GET /api/gentasks/v1/tasks/{token}", httpapi.RequirePermission(access.PermissionBoardRead), handler.findTask)
	router.HandleFunc("PATCH /api/gentasks/v1/tasks/{token}", httpapi.RequirePermission(access.PermissionTaskWrite), handler.updateTask)
	router.HandleFunc("POST /api/gentasks/v1/tasks/{token}/move", httpapi.RequirePermission(access.PermissionTaskWrite), handler.moveTask)
	router.HandleFunc("POST /api/gentasks/v1/tasks/{token}/archive", httpapi.RequirePermission(access.PermissionTaskWrite), handler.archiveTask)
	router.HandleFunc("POST /api/gentasks/v1/tasks/{token}/unarchive", httpapi.RequirePermission(access.PermissionTaskWrite), handler.unarchiveTask)
	router.HandleFunc("DELETE /api/gentasks/v1/tasks/{token}", httpapi.RequirePermission(access.PermissionTaskDelete), handler.deleteTask)
}

func callerFrom(r *http.Request) Caller {
	principal, _ := httpapi.PrincipalFrom(r.Context())
	return Caller{UserToken: principal.UserToken, AccessToken: principal.AccessToken}
}

func (handler *TaskHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	archived := r.URL.Query().Get("archived") == "true"
	listed, err := handler.service.ListTasks(r.Context(), r.PathValue("token"), archived)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, TaskListResponse{Tasks: NewTaskResponses(listed)})
}

func (handler *TaskHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var request CreateTaskRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	task, err := handler.service.CreateTask(r.Context(), callerFrom(r), r.PathValue("token"), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewTaskResponse(task))
}

func (handler *TaskHandler) findTask(w http.ResponseWriter, r *http.Request) {
	task, err := handler.service.FindTask(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewTaskResponse(task))
}

func (handler *TaskHandler) updateTask(w http.ResponseWriter, r *http.Request) {
	var request UpdateTaskRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	changes, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	task, err := handler.service.UpdateTask(r.Context(), callerFrom(r), r.PathValue("token"), changes)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewTaskResponse(task))
}

func (handler *TaskHandler) moveTask(w http.ResponseWriter, r *http.Request) {
	var request MoveTaskRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	columnToken, position, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.service.MoveTask(r.Context(), r.PathValue("token"), columnToken, position); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *TaskHandler) archiveTask(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.ArchiveTask(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *TaskHandler) unarchiveTask(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.UnarchiveTask(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *TaskHandler) deleteTask(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteTask(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
