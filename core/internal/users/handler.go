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

// DirectoryUserResponse is the public view of a user that clients and modules may reference (rule 14).
type DirectoryUserResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type DirectoryResponse struct {
	Users []DirectoryUserResponse `json:"users"`
}

func NewDirectoryUserResponse(user User) DirectoryUserResponse {
	return DirectoryUserResponse{Token: user.Token, Name: user.Name, Email: user.Email}
}

type Handler struct {
	db      *pgxpool.Pool
	users   Repository
	service *Service
	access  *access.Service
}

func NewHandler(db *pgxpool.Pool, service *Service, accessService *access.Service) *Handler {
	return &Handler{db: db, service: service, access: accessService}
}

func (handler *Handler) Register(router *platform.Router) {
	router.HandleFunc("GET /api/core/v1/me", platform.Authenticated, handler.me)
	router.HandleFunc("GET /api/core/v1/users", platform.RequirePermission(access.PermissionUserRead), handler.listDirectory)
	router.HandleFunc("GET /api/core/v1/users/{token}", platform.RequirePermission(access.PermissionUserRead), handler.findInDirectory)
}

func (handler *Handler) listDirectory(w http.ResponseWriter, r *http.Request) {
	directory, err := handler.service.ListDirectory(r.Context(), handler.db)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	response := DirectoryResponse{Users: make([]DirectoryUserResponse, 0, len(directory))}
	for _, user := range directory {
		response.Users = append(response.Users, NewDirectoryUserResponse(user))
	}
	platform.WriteJSON(w, http.StatusOK, response)
}

func (handler *Handler) findInDirectory(w http.ResponseWriter, r *http.Request) {
	user, err := handler.service.FindInDirectory(r.Context(), handler.db, r.PathValue("token"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, NewDirectoryUserResponse(user))
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
