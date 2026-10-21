package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
)

type policyKind int

const (
	policyUndeclared policyKind = iota
	policyPublic
	policyAuthenticated
	policyPermission
)

type Policy struct {
	kind       policyKind
	permission string
}

var (
	Public        = Policy{kind: policyPublic}
	Authenticated = Policy{kind: policyAuthenticated}
)

func RequirePermission(permission string) Policy {
	return Policy{kind: policyPermission, permission: permission}
}

type AccessTokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (tokens.Principal, error)
}

type Router struct {
	mux      *http.ServeMux
	verifier AccessTokenVerifier
}

func NewRouter(verifier AccessTokenVerifier) *Router {
	return &Router{mux: http.NewServeMux(), verifier: verifier}
}

// Handle panics on an undeclared policy so that a route can never become public by omission (rule 18).
func (router *Router) Handle(pattern string, policy Policy, handler http.Handler) {
	switch policy.kind {
	case policyPublic:
		router.mux.Handle(pattern, handler)
	case policyAuthenticated:
		router.mux.Handle(pattern, router.requireAuthentication(handler))
	case policyPermission:
		if policy.permission == "" {
			panic(fmt.Sprintf("route %q requires a permission name", pattern))
		}
		router.mux.Handle(pattern, router.requireAuthentication(requirePermission(policy.permission, handler)))
	default:
		panic(fmt.Sprintf("route %q has no access policy", pattern))
	}
}

func (router *Router) HandleFunc(pattern string, policy Policy, handler http.HandlerFunc) {
	router.Handle(pattern, policy, handler)
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}

func (router *Router) requireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found {
			WriteError(w, r, ErrInvalidToken)
			return
		}
		principal, err := router.verifier.Verify(r.Context(), strings.TrimSpace(rawToken))
		if errors.Is(err, tokens.ErrKeysUnavailable) {
			WriteError(w, r, ErrServiceUnavailable)
			return
		}
		if err != nil {
			WriteError(w, r, ErrInvalidToken)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), principal)))
	})
}

func requirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, _ := PrincipalFrom(r.Context())
		if !principal.HasPermission(permission) {
			WriteError(w, r, ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal tokens.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (tokens.Principal, bool) {
	principal, found := ctx.Value(principalContextKey{}).(tokens.Principal)
	return principal, found
}
