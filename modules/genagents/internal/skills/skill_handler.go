package skills

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/access"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type SkillFileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type SkillResponse struct {
	Token       string              `json:"token"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Body        string              `json:"body"`
	Version     int                 `json:"version"`
	Files       []SkillFileResponse `json:"files"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

type SkillListResponse struct {
	Skills []SkillResponse `json:"skills"`
}

func NewSkillResponse(skill Skill) SkillResponse {
	files := make([]SkillFileResponse, 0, len(skill.Files))
	for _, file := range skill.Files {
		files = append(files, SkillFileResponse{Path: file.Path, Content: file.Content})
	}
	return SkillResponse{
		Token: skill.Token, Name: skill.Name, Description: skill.Description, Body: skill.Body, Version: skill.Version,
		Files: files, UpdatedAt: skill.UpdatedAt,
	}
}

type SkillHandler struct {
	service *SkillService
}

func NewSkillHandler(service *SkillService) *SkillHandler {
	return &SkillHandler{service: service}
}

func (handler *SkillHandler) Register(router *httpapi.Router) {
	manage := httpapi.RequirePermission(access.PermissionSkillManage)
	router.HandleFunc("GET /api/genagents/v1/skills", manage, handler.listSkills)
	router.HandleFunc("POST /api/genagents/v1/skills", manage, handler.createSkill)
	router.HandleFunc("POST /api/genagents/v1/skills/import", manage, handler.importSkill)
	router.HandleFunc("GET /api/genagents/v1/skills/{token}", manage, handler.findSkill)
	router.HandleFunc("PATCH /api/genagents/v1/skills/{token}", manage, handler.updateSkill)
	router.HandleFunc("DELETE /api/genagents/v1/skills/{token}", manage, handler.deleteSkill)
}

func (handler *SkillHandler) listSkills(w http.ResponseWriter, r *http.Request) {
	listed, err := handler.service.ListSkills(r.Context())
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	response := SkillListResponse{Skills: make([]SkillResponse, 0, len(listed))}
	for _, skill := range listed {
		response.Skills = append(response.Skills, NewSkillResponse(skill))
	}
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (handler *SkillHandler) createSkill(w http.ResponseWriter, r *http.Request) {
	var request CreateSkillRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	skill, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	handler.writeCreated(w, r, skill)
}

func (handler *SkillHandler) importSkill(w http.ResponseWriter, r *http.Request) {
	var request ImportSkillRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	skill, err := request.Validate()
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	handler.writeCreated(w, r, skill)
}

func (handler *SkillHandler) writeCreated(w http.ResponseWriter, r *http.Request, skill Skill) {
	created, err := handler.service.CreateSkill(r.Context(), skill)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, NewSkillResponse(created))
}

func (handler *SkillHandler) findSkill(w http.ResponseWriter, r *http.Request) {
	skill, err := handler.service.FindSkill(r.Context(), r.PathValue("token"))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewSkillResponse(skill))
}

func (handler *SkillHandler) updateSkill(w http.ResponseWriter, r *http.Request) {
	var request UpdateSkillRequest
	if err := httpapi.DecodeJSON(w, r, &request); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	updated, err := handler.service.UpdateSkill(r.Context(), r.PathValue("token"), request)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, NewSkillResponse(updated))
}

func (handler *SkillHandler) deleteSkill(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.DeleteSkill(r.Context(), r.PathValue("token")); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
