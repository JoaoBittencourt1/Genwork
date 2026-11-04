package passwordreset

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

type RequestResetRequest struct {
	Email string `json:"email"`
}

type ConfirmResetRequest struct {
	Password string `json:"password"`
}

type Handler struct {
	service *Service
	limiter auth.Middleware
}

func NewHandler(service *Service, limiter auth.Middleware) *Handler {
	return &Handler{service: service, limiter: limiter}
}

func (handler *Handler) Register(router *platform.Router) {
	router.Handle("POST /api/core/v1/auth/password-reset", platform.Public, handler.limiter(http.HandlerFunc(handler.request)))
	router.Handle("POST /api/core/v1/auth/password-reset/{secret}", platform.Public, handler.limiter(http.HandlerFunc(handler.confirm)))
}

func (handler *Handler) request(w http.ResponseWriter, r *http.Request) {
	var body RequestResetRequest
	if err := platform.DecodeJSON(w, r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if err := handler.service.Request(r.Context(), body.Email); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	var body ConfirmResetRequest
	if err := platform.DecodeJSON(w, r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if err := handler.service.Confirm(r.Context(), r.PathValue("secret"), body.Password); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
