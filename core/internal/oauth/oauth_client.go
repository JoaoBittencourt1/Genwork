// Package oauth is core's OAuth 2.1 authorization server for AI assistants: public clients with PKCE, whose grants
// are ordinary sessions of the oauth client type (design §11).
package oauth

import (
	"slices"
	"time"
)

// OAuthClient is a dynamically registered public client; Token is its client_id (rule 14).
type OAuthClient struct {
	ID           int64
	Token        string
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

// HasRedirectURI compares exactly, as OAuth 2.1 requires: no prefix or pattern matching.
func (client OAuthClient) HasRedirectURI(uri string) bool {
	return slices.Contains(client.RedirectURIs, uri)
}
