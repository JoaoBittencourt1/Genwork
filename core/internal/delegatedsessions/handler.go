package delegatedsessions

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/sessions"
)

type DelegatedSessionHandler struct {
	service *DelegatedSessionService
}

func NewDelegatedSessionHandler(service *DelegatedSessionService) *DelegatedSessionHandler {
	return &DelegatedSessionHandler{service: service}
}

// Register: a person signed in on the web or a native app may grant a delegated session for themselves, never for
// another member (rule 18). Bot, OAuth, and delegated sessions cannot, so a short-lived or limited token never turns
// into a renewable 7-day one.
func (handler *DelegatedSessionHandler) Register(router *platform.Router) {
	router.HandleFunc("POST /api/core/v1/delegated-sessions", platform.Authenticated, handler.grant)
}

func (handler *DelegatedSessionHandler) grant(w http.ResponseWriter, r *http.Request) {
	var request GrantRequest
	if err := platform.DecodeJSON(w, r, &request); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	input, err := request.ToGrantInput()
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	principal, _ := platform.PrincipalFrom(r.Context())
	if !sessions.ClientType(principal.ClientType).SignsInWithPassword() {
		platform.WriteError(w, r, platform.ErrForbidden)
		return
	}
	signIn, err := handler.service.Grant(r.Context(), principal, input)
	if err != nil {
		platform.WriteError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	auth.WriteSignIn(w, signIn)
}
