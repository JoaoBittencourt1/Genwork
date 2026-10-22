package oauth

import (
	"net/http"

	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
)

// ProtocolError is answered in the RFC 6749 shape ({error, error_description}) by the endpoints OAuth clients call,
// registration and token, because clients parse that shape. Every other endpoint keeps the platform envelope.
type ProtocolError struct {
	Status      int
	Code        string
	Description string
}

func (err *ProtocolError) Error() string { return err.Code + ": " + err.Description }

func newProtocolError(code, description string) *ProtocolError {
	return &ProtocolError{Status: http.StatusBadRequest, Code: code, Description: description}
}

var (
	errInvalidGrant         = newProtocolError("invalid_grant", "the grant is invalid, expired, or was issued to another client")
	errUnsupportedGrantType = newProtocolError("unsupported_grant_type", "only authorization_code and refresh_token are supported")
	errUnknownClient        = newProtocolError("invalid_client", "unknown client_id")
)

// Errors of an authorization request that core reports to the user instead of redirecting, since the redirect target
// cannot be trusted (RFC 6749 §4.1.2.1). The consent page reads them in the platform envelope.
var (
	ErrUnknownClient           = platform.NewAPIError(http.StatusBadRequest, "invalid_client")
	ErrUnregisteredRedirectURI = platform.NewAPIError(http.StatusBadRequest, "invalid_redirect_uri")
)
