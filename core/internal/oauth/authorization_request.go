package oauth

import (
	"net/url"
	"regexp"
	"strings"
)

// Scope is the only scope: acting as the signed-in user, with that user's permissions.
const Scope = "genwork"

var pkceValuePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

// AuthorizationRequest carries the parameters of /oauth/authorize. The consent page's preview and its decision both
// validate through it, so the rules live once (design §11).
type AuthorizationRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
	Resource            string
}

// redirectableError is an error the client learns through its redirect URI, with the request's state.
type redirectableError struct {
	code string
}

// validateRedirectable checks what is reported to the client; the client and redirect URI were checked before.
func (request AuthorizationRequest) validateRedirectable(resource string) *redirectableError {
	switch {
	case request.ResponseType != "code":
		return &redirectableError{code: "unsupported_response_type"}
	case !pkceValuePattern.MatchString(request.CodeChallenge) || request.CodeChallengeMethod != "S256":
		return &redirectableError{code: "invalid_request"}
	case !isGenworkScope(request.Scope):
		return &redirectableError{code: "invalid_scope"}
	case request.Resource != "" && request.Resource != resource:
		return &redirectableError{code: "invalid_target"}
	}
	return nil
}

// isGenworkScope accepts an omitted scope, which means the default one.
func isGenworkScope(scope string) bool {
	for _, requested := range strings.Fields(scope) {
		if requested != Scope {
			return false
		}
	}
	return true
}

// redirectTo adds the parameters to the registered redirect URI, keeping any query it already has.
func redirectTo(redirectURI string, parameters url.Values) string {
	target, err := url.Parse(redirectURI)
	if err != nil {
		return redirectURI
	}
	query := target.Query()
	for name, values := range parameters {
		for _, value := range values {
			if value != "" {
				query.Add(name, value)
			}
		}
	}
	target.RawQuery = query.Encode()
	return target.String()
}
