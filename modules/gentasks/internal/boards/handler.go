package boards

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type BoardResponse struct {
	Token      string    `json:"token"`
	Name       string    `json:"name"`
	TaskPrefix string    `json:"taskPrefix"`
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type BoardListResponse struct {
	Boards []BoardResponse `json:"boards"`
}

type ColumnResponse struct {
	Token    string        `json:"token"`
	Preset   *ColumnPreset `json:"preset"`
	Name     *string       `json:"name"`
	Position int           `json:"position"`
}

type CreatedBoardResponse struct {
	BoardResponse
	Columns []ColumnResponse `json:"columns"`
}

func NewBoardResponse(board Board) BoardResponse {
	return BoardResponse{
		Token: board.Token, Name: board.Name, TaskPrefix: board.TaskPrefix,
		CreatedBy: board.CreatedBy, CreatedAt: board.CreatedAt, UpdatedAt: board.UpdatedAt,
	}
}

func NewColumnResponse(column Column) ColumnResponse {
	return ColumnResponse{Token: column.Token, Preset: column.Preset, Name: column.Name, Position: column.Position}
}

type BoardHandler struct {
	service *BoardService
}

func NewBoardHandler(service *BoardService) *BoardHandler {
	return &BoardHandler{service: service}
}

func (handler *BoardHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/boards", httpapi.RequirePermission(access.PermissionBoardRead), handler.listBoards)
	router.HandleFunc("POST /api/gentasks/v1/boards", httpapi.RequirePermission(access.PermissionBoardManage), handler.createBoard)
	router.HandleFunc("PATCH /api/gentasks/v1/boards/{token}", httpapi.RequirePermission(access.PermissionBoardManage), handler.updateBoard)
	router.HandleFunc("DELETE /api/gentasks/v1/boards/{token}", httpapi.RequirePermission(access.PermissionBoardManage), handler.deleteBoard)
	router.HandleFunc("POST /api/gentasks/v1/boards/{token}/columns", httpapi.RequirePermission(access.PermissionBoardManage), handler.addColumn)
	router.HandleFunc("PATCH /api/gentasks/v1/columns/{token}", httpapi.RequirePermission(access.PermissionBoardManage), handler.updateColumn)
	router.HandleFunc("POST /api/gentasks/v1/columns/{token}/move", httpapi.RequirePermission(access.PermissionBoardManage), handler.moveColumn)
	router.HandleFunc("DELETE /api/gentasks/v1/columns/{token}", httpapi.RequirePermission(access.PermissionBoardManage), handler.deleteColumn)
}

func (handler *BoardHandler) listBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := handler.service.ListBoards(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := BoardListResponse{Boards: make([]BoardResponse, 0, len(boards))}
	for _, board := range boards {
		response.Boards = append(response.Boards, NewBoardResponse(board))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *BoardHandler) createBoard(w http.ResponseWriter, r *http.Request) {
	var request CreateBoardRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	principal, _ := httpapi.PrincipalFrom(r.Context())
	board, columns, err := handler.service.CreateBoard(r.Context(), input, principal.UserToken)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := CreatedBoardResponse{BoardResponse: NewBoardResponse(board), Columns: make([]ColumnResponse, 0, len(columns))}
	for _, column := range columns {
		response.Columns = append(response.Columns, NewColumnResponse(column))
	}
	httpapi.WriteJSON(w, http.StatusCreated, response)
}

func (handler *BoardHandler) updateBoard(w http.ResponseWriter, r *http.Request) {
	var request UpdateBoardRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	changes, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	board, err := handler.service.UpdateBoard(r.Context(), r.PathValue("token"), changes)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewBoardResponse(board))
}

func (handler *BoardHandler) deleteBoard(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteBoard(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *BoardHandler) addColumn(w http.ResponseWriter, r *http.Request) {
	var request CreateColumnRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	name, err := request.NormalizedName()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	column, err := handler.service.AddColumn(r.Context(), r.PathValue("token"), name)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewColumnResponse(column))
}

func (handler *BoardHandler) updateColumn(w http.ResponseWriter, r *http.Request) {
	var request UpdateColumnRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	change, err := request.NameChange()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	column, err := handler.service.UpdateColumn(r.Context(), r.PathValue("token"), change)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewColumnResponse(column))
}

func (handler *BoardHandler) moveColumn(w http.ResponseWriter, r *http.Request) {
	var request MoveColumnRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	position, err := request.TargetPosition()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := handler.service.MoveColumn(r.Context(), r.PathValue("token"), position); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *BoardHandler) deleteColumn(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteColumn(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
