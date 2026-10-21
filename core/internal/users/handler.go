package users

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MeResponse struct {
	Token       string              `json:"token"`
	Email       string              `json:"email"`
	Name        string              `json:"name"`
	Role        access.RoleResponse `json:"role"`
	Permissions []string            `json:"permissions"`
}

type Handler struct {
	db     *pgxpool.Pool
	users  Repository
	access *access.Service
}

func NewHandler(db *pgxpool.Pool, accessService *access.Service) *Handler {
	return &Handler{db: db, access: accessService}
}

func (handler *Handler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/me", platform.Authenticated, handler.me)
}

func (handler *Handler) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := platform.PrincipalFrom(r.Context())
	user, err := handler.users.FindByID(r.Context(), handler.db, principal.UserID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	role, err := handler.access.FindRoleByID(r.Context(), user.RoleID)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, MeResponse{
		Token:       user.Token,
		Email:       user.Email,
		Name:        user.Name,
		Role:        access.NewRoleResponse(role),
		Permissions: role.Permissions,
	})
}
