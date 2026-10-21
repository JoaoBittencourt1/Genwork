package labels

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type LabelResponse struct {
	Token string     `json:"token"`
	Name  string     `json:"name"`
	Color LabelColor `json:"color"`
}

type LabelListResponse struct {
	Labels []LabelResponse `json:"labels"`
}

func NewLabelResponse(label Label) LabelResponse {
	return LabelResponse{Token: label.Token, Name: label.Name, Color: label.Color}
}

func NewLabelResponses(labels []Label) []LabelResponse {
	responses := make([]LabelResponse, 0, len(labels))
	for _, label := range labels {
		responses = append(responses, NewLabelResponse(label))
	}
	return responses
}

type LabelHandler struct {
	service *LabelService
}

func NewLabelHandler(service *LabelService) *LabelHandler {
	return &LabelHandler{service: service}
}

func (handler *LabelHandler) Register(router *httpapi.Router) {
	router.HandleFunc("GET /api/gentasks/v1/boards/{token}/labels", httpapi.RequirePermission(access.PermissionBoardRead), handler.listLabels)
	router.HandleFunc("POST /api/gentasks/v1/boards/{token}/labels", httpapi.RequirePermission(access.PermissionBoardManage), handler.createLabel)
	router.HandleFunc("PATCH /api/gentasks/v1/labels/{token}", httpapi.RequirePermission(access.PermissionBoardManage), handler.updateLabel)
	router.HandleFunc("DELETE /api/gentasks/v1/labels/{token}", httpapi.RequirePermission(access.PermissionBoardManage), handler.deleteLabel)
}

func (handler *LabelHandler) listLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := handler.service.ListLabels(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, LabelListResponse{Labels: NewLabelResponses(labels)})
}

func (handler *LabelHandler) createLabel(w http.ResponseWriter, r *http.Request) {
	var request CreateLabelRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	input, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	label, err := handler.service.CreateLabel(r.Context(), r.PathValue("token"), input)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewLabelResponse(label))
}

func (handler *LabelHandler) updateLabel(w http.ResponseWriter, r *http.Request) {
	var request UpdateLabelRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	changes, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	label, err := handler.service.UpdateLabel(r.Context(), r.PathValue("token"), changes)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewLabelResponse(label))
}

func (handler *LabelHandler) deleteLabel(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteLabel(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
