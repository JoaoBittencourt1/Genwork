package platform

import "context"

type Principal struct {
	UserID       int64
	UserToken    string
	RoleID       int64
	SessionID    int64
	SessionToken string
	// ClientType is the kind of session the request came in on ("web", "native", "oauth", "bot", "delegated").
	ClientType string
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	principal, found := ctx.Value(principalContextKey{}).(Principal)
	return principal, found
}
