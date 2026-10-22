// Package remote serves Genwork's MCP tools over Streamable HTTP at <public URL>/mcp, so assistants connect with a URL
// and a browser sign-in instead of a locally installed binary (design §12).
package remote

import (
	"errors"
	"net/url"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/mcp/internal/platform"
)

type HTTPServerConfig struct {
	Address string
	// PublicURL is the token issuer and the base of the resource URL clients see.
	PublicURL string
	// CoreInternalURL serves core's signing keys; GatewayInternalURL is where tool calls reach the Genwork APIs.
	CoreInternalURL    string
	GatewayInternalURL string
}

// LoadHTTPServerConfig fails at startup instead of falling back to defaults (rule 1).
func LoadHTTPServerConfig(getenv platform.Getenv) (HTTPServerConfig, error) {
	read := func(name string) string { return strings.TrimRight(strings.TrimSpace(getenv(name)), "/") }
	config := HTTPServerConfig{
		Address: strings.TrimSpace(getenv("MCP_HTTP_ADDR")), PublicURL: read("GENWORK_PUBLIC_URL"),
		CoreInternalURL: read("CORE_INTERNAL_URL"), GatewayInternalURL: read("GATEWAY_INTERNAL_URL"),
	}
	var missing, invalid []string
	for _, variable := range []struct {
		name, value string
		isURL       bool
	}{
		{"MCP_HTTP_ADDR", config.Address, false},
		{"GENWORK_PUBLIC_URL", config.PublicURL, true},
		{"CORE_INTERNAL_URL", config.CoreInternalURL, true},
		{"GATEWAY_INTERNAL_URL", config.GatewayInternalURL, true},
	} {
		switch {
		case variable.value == "":
			missing = append(missing, variable.name)
		case variable.isURL && !isAbsoluteHTTPURL(variable.value):
			invalid = append(invalid, variable.name)
		}
	}
	var problems []string
	if len(missing) > 0 {
		problems = append(problems, "missing required environment variables: "+strings.Join(missing, ", "))
	}
	if len(invalid) > 0 {
		problems = append(problems, "not absolute http or https URLs: "+strings.Join(invalid, ", "))
	}
	if len(problems) > 0 {
		return HTTPServerConfig{}, errors.New(strings.Join(problems, "; "))
	}
	return config, nil
}

func isAbsoluteHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
