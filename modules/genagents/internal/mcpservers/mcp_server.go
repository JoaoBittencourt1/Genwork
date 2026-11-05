package mcpservers

import (
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const (
	MaxNameLength = 80
	MaxEntries    = 30
	MaxArgs       = 50
	MaxFieldBytes = 4096
)

type Transport string

const (
	TransportHTTP  Transport = "http"
	TransportStdio Transport = "stdio"
)

var (
	ErrMCPServerNotFound  = httpapi.NewAPIError(http.StatusNotFound, "mcp_server_not_found")
	ErrMCPServerNameTaken = httpapi.NewAPIError(http.StatusConflict, "mcp_server_name_taken")
)

// Entry is a header or an environment variable. A secret entry's value lives only in the encrypted secrets column; an
// entry read back from storage carries HasValue instead of the value.
type Entry struct {
	Name     string `json:"name"`
	Secret   bool   `json:"secret"`
	Value    string `json:"value,omitempty"`
	HasValue bool   `json:"hasValue,omitempty"`
}

// Config is what is stored in clear: no secret value is ever part of it.
type Config struct {
	URL     string   `json:"url,omitempty"`
	Headers []Entry  `json:"headers,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []Entry  `json:"env,omitempty"`
}

// McpServer is an external MCP server an admin registers. Secrets maps "header:Name" and "env:NAME" to their values and
// is filled only when a run needs them.
type McpServer struct {
	ID        int64
	Token     string
	Name      string
	Transport Transport
	Config    Config
	Secrets   map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func SecretKey(kind, name string) string {
	return kind + ":" + name
}

func IsValidName(name string) bool {
	length := utf8.RuneCountInString(name)
	return length > 0 && length <= MaxNameLength
}

// Validate checks the transport-specific fields; the secret values themselves are validated by the caller.
func (server McpServer) Validate() map[string]string {
	fields := map[string]string{}
	if !IsValidName(server.Name) {
		fields["name"] = "invalid"
	}
	switch server.Transport {
	case TransportHTTP:
		if !isHTTPURL(server.Config.URL) {
			fields["url"] = "invalid"
		}
		validateEntries(fields, "headers", server.Config.Headers)
	case TransportStdio:
		if strings.TrimSpace(server.Config.Command) == "" || len(server.Config.Command) > MaxFieldBytes {
			fields["command"] = "invalid"
		}
		if len(server.Config.Args) > MaxArgs {
			fields["args"] = "invalid"
		}
		validateEntries(fields, "env", server.Config.Env)
	default:
		fields["transport"] = "invalid"
	}
	return fields
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && len(raw) <= MaxFieldBytes
}

func validateEntries(fields map[string]string, field string, entries []Entry) {
	if len(entries) > MaxEntries {
		fields[field] = "invalid"
		return
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" || len(entry.Name) > 200 || seen[entry.Name] || len(entry.Value) > MaxFieldBytes {
			fields[field] = "invalid"
			return
		}
		seen[entry.Name] = true
	}
}
