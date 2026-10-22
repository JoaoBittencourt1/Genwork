package remote

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/mcp/internal/toolset"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/tokens"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const scope = "genwork"

type HandlerDependencies struct {
	Version    string
	HTTPClient *http.Client
	// Toolsets lists the tools this server knows, as the stdio server does.
	Toolsets     func(api platform.APIRequester) []toolset.Toolset
	SyncInterval time.Duration
	Now          func() time.Time
}

// NewHandler serves /mcp behind bearer authentication, plus the RFC 9728 metadata that tells clients which
// authorization server issues its tokens: core, at the instance's public URL.
func NewHandler(config HTTPServerConfig, dependencies HandlerDependencies) http.Handler {
	resourceMetadataURL := config.PublicURL + "/.well-known/oauth-protected-resource/mcp"
	metadata := auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               config.PublicURL + "/mcp",
		AuthorizationServers:   []string{config.PublicURL},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        []string{scope},
		ResourceName:           "Genwork",
	})
	verifier := tokens.NewTokenVerifier(tokens.TokenVerifierConfig{
		JWKSURL: config.CoreInternalURL + "/api/core/v1/auth/jwks", Issuer: config.PublicURL, HTTPClient: dependencies.HTTPClient,
	})
	factory := sessionServerFactory{dependencies: dependencies, gatewayURL: config.GatewayInternalURL}
	sessions := mcp.NewStreamableHTTPHandler(factory.newSessionServer, &mcp.StreamableHTTPOptions{})
	requireToken := auth.RequireBearerToken(verifyBearerToken(verifier), &auth.RequireBearerTokenOptions{ResourceMetadataURL: resourceMetadataURL})

	mux := http.NewServeMux()
	mux.Handle("/.well-known/oauth-protected-resource", metadata)
	mux.Handle("/.well-known/oauth-protected-resource/mcp", metadata)
	mux.Handle("/mcp", requireToken(sessions))
	return mux
}
