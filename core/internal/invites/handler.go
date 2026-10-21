package invites

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/access"
	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
)

type CreateRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type AcceptRequest struct {
	Name       string  `json:"name"`
	Password   string  `json:"password"`
	Client     string  `json:"client"`
	DeviceName *string `json:"deviceName"`
}

type RoleSummaryResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type InviterResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type InviteResponse struct {
	Token     string              `json:"token"`
	Email     string              `json:"email"`
	Role      RoleSummaryResponse `json:"role"`
	InvitedBy InviterResponse     `json:"invitedBy"`
	Status    Status              `json:"status"`
	CreatedAt time.Time           `json:"createdAt"`
	ExpiresAt time.Time           `json:"expiresAt"`
}

type DeliveryResponse struct {
	Invite    InviteResponse `json:"invite"`
	Link      string         `json:"link"`
	EmailSent bool           `json:"emailSent"`
}

type InviteListResponse struct {
	Invites []InviteResponse `json:"invites"`
}

type InspectionResponse struct {
	InstanceName string    `json:"instanceName"`
	Email        string    `json:"email"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type Handler struct {
	service           *Service
	invitationLimiter auth.Middleware
	now               func() time.Time
}

func NewHandler(service *Service, invitationLimiter auth.Middleware, now func() time.Time) *Handler {
	return &Handler{service: service, invitationLimiter: invitationLimiter, now: now}
}

func (handler *Handler) Register(router *platform.Router) {
	router.HandleFunc("POST /api/core/v1/invites", platform.RequirePermission(access.PermissionInviteCreate), handler.create)
	router.HandleFunc("GET /api/core/v1/invites", platform.RequirePermission(access.PermissionInviteRead), handler.list)
	router.HandleFunc("DELETE /api/core/v1/invites/{token}", platform.RequirePermission(access.PermissionInviteRevoke), handler.revoke)
	router.HandleFunc("POST /api/core/v1/invites/{token}/resend", platform.RequirePermission(access.PermissionInviteCreate), handler.resend)
	router.Handle("GET /api/core/v1/invitations/{secret}", platform.Public, handler.invitationLimiter(http.HandlerFunc(handler.inspect)))
	router.Handle("POST /api/core/v1/invitations/{secret}/accept", platform.Public, handler.invitationLimiter(http.HandlerFunc(handler.accept)))
}

func (handler *Handler) create(w http.ResponseWriter, r *http.Request) {
	var request CreateRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	principal, _ := platform.PrincipalFrom(r.Context())
	delivery, err := handler.service.Create(r.Context(), principal, CreateInput{Email: request.Email, RoleToken: request.Role})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusCreated, handler.newDeliveryResponse(delivery))
}

func (handler *Handler) list(w http.ResponseWriter, r *http.Request) {
	invites, err := handler.service.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	now := handler.now()
	response := InviteListResponse{Invites: make([]InviteResponse, 0, len(invites))}
	for _, invite := range invites {
		response.Invites = append(response.Invites, newInviteResponse(invite, now))
	}
	platform.WriteJSON(w, http.StatusOK, response)
}

func (handler *Handler) revoke(w http.ResponseWriter, r *http.Request) {
	if err := handler.service.Revoke(r.Context(), r.PathValue("token")); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) resend(w http.ResponseWriter, r *http.Request) {
	delivery, err := handler.service.Resend(r.Context(), r.PathValue("token"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, handler.newDeliveryResponse(delivery))
}

func (handler *Handler) inspect(w http.ResponseWriter, r *http.Request) {
	inspection, err := handler.service.Inspect(r.Context(), r.PathValue("secret"))
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	platform.WriteJSON(w, http.StatusOK, InspectionResponse{
		InstanceName: inspection.InstanceName, Email: inspection.Email, ExpiresAt: inspection.ExpiresAt,
	})
}

func (handler *Handler) accept(w http.ResponseWriter, r *http.Request) {
	var request AcceptRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	signIn, err := handler.service.Accept(r.Context(), r.PathValue("secret"), AcceptInput{
		Name:        request.Name,
		Password:    request.Password,
		ClientInput: auth.ClientInput{Client: sessions.ClientType(request.Client), DeviceName: request.DeviceName},
	})
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	auth.WriteSignIn(w, signIn)
}

func (handler *Handler) newDeliveryResponse(delivery Delivery) DeliveryResponse {
	return DeliveryResponse{
		Invite:    newInviteResponse(delivery.Invite, handler.now()),
		Link:      delivery.Link,
		EmailSent: delivery.EmailSent,
	}
}

func newInviteResponse(invite Invite, now time.Time) InviteResponse {
	return InviteResponse{
		Token:     invite.Token,
		Email:     invite.Email,
		Role:      RoleSummaryResponse{Token: invite.RoleToken, Name: invite.RoleName},
		InvitedBy: InviterResponse{Token: invite.InviterToken, Name: invite.InviterName},
		Status:    invite.StatusAt(now),
		CreatedAt: invite.CreatedAt,
		ExpiresAt: invite.ExpiresAt,
	}
}
