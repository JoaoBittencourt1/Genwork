package mcpservers

import (
	"strings"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

type EntryRequest struct {
	Name   string `json:"name"`
	Secret bool   `json:"secret"`
	Value  string `json:"value"`
}

type CreateMcpServerRequest struct {
	Name      string         `json:"name"`
	Transport string         `json:"transport"`
	URL       string         `json:"url"`
	Headers   []EntryRequest `json:"headers"`
	Command   string         `json:"command"`
	Args      []string       `json:"args"`
	Env       []EntryRequest `json:"env"`
}

func toEntries(requested []EntryRequest) []Entry {
	entries := make([]Entry, 0, len(requested))
	for _, entry := range requested {
		entries = append(entries, Entry{Name: strings.TrimSpace(entry.Name), Secret: entry.Secret, Value: entry.Value})
	}
	return entries
}

// Validate maps the request and checks it; every secret entry must carry its value on creation.
func (request CreateMcpServerRequest) Validate() (McpServer, error) {
	server := McpServer{
		Name: strings.TrimSpace(request.Name), Transport: Transport(request.Transport),
		Config: Config{URL: strings.TrimSpace(request.URL), Headers: toEntries(request.Headers), Command: strings.TrimSpace(request.Command), Args: request.Args, Env: toEntries(request.Env)},
	}
	fields := server.Validate()
	for _, entries := range []struct {
		field   string
		entries []Entry
	}{{"headers", server.Config.Headers}, {"env", server.Config.Env}} {
		for _, entry := range entries.entries {
			if entry.Secret && entry.Value == "" {
				fields[entries.field] = "secret_value_required"
			}
		}
	}
	if len(fields) > 0 {
		return McpServer{}, httpapi.NewValidationError(fields)
	}
	return server, nil
}

// UpdateMcpServerRequest is a PATCH body: omitted fields stay unchanged and none accepts null (rule 17). A secret entry
// sent without a value keeps the value stored for that name.
type UpdateMcpServerRequest struct {
	Name    httpapi.PatchField[string]         `json:"name"`
	URL     httpapi.PatchField[string]         `json:"url"`
	Headers httpapi.PatchField[[]EntryRequest] `json:"headers"`
	Command httpapi.PatchField[string]         `json:"command"`
	Args    httpapi.PatchField[[]string]       `json:"args"`
	Env     httpapi.PatchField[[]EntryRequest] `json:"env"`
}

func (request UpdateMcpServerRequest) rejectNulls() error {
	fields := map[string]string{}
	for name, null := range map[string]bool{
		"name": request.Name.Null, "url": request.URL.Null, "headers": request.Headers.Null,
		"command": request.Command.Null, "args": request.Args.Null, "env": request.Env.Null,
	} {
		if null {
			fields[name] = "not_nullable"
		}
	}
	if len(fields) > 0 {
		return httpapi.NewValidationError(fields)
	}
	return nil
}

// ApplyTo returns the server with the present fields replaced; existingSecrets lets secret entries keep their value.
func (request UpdateMcpServerRequest) ApplyTo(server McpServer, existingSecrets map[string]string) (McpServer, error) {
	if err := request.rejectNulls(); err != nil {
		return McpServer{}, err
	}
	secrets := map[string]string{}
	for key, value := range existingSecrets {
		secrets[key] = value
	}
	if request.Name.Present {
		server.Name = strings.TrimSpace(request.Name.Value)
	}
	if request.URL.Present {
		server.Config.URL = strings.TrimSpace(request.URL.Value)
	}
	if request.Command.Present {
		server.Config.Command = strings.TrimSpace(request.Command.Value)
	}
	if request.Args.Present {
		server.Config.Args = request.Args.Value
	}
	if request.Headers.Present {
		server.Config.Headers = toEntries(request.Headers.Value)
		secrets = withoutPrefix(secrets, "header:")
		for key, value := range replacementSecrets("header", server.Config.Headers, existingSecrets) {
			secrets[key] = value
		}
	}
	if request.Env.Present {
		server.Config.Env = toEntries(request.Env.Value)
		secrets = withoutPrefix(secrets, "env:")
		for key, value := range replacementSecrets("env", server.Config.Env, existingSecrets) {
			secrets[key] = value
		}
	}
	fields := server.Validate()
	for _, kind := range []struct {
		field   string
		prefix  string
		entries []Entry
	}{{"headers", "header", server.Config.Headers}, {"env", "env", server.Config.Env}} {
		for _, entry := range kind.entries {
			if _, known := secrets[SecretKey(kind.prefix, entry.Name)]; entry.Secret && !known {
				fields[kind.field] = "secret_value_required"
			}
		}
	}
	if len(fields) > 0 {
		return McpServer{}, httpapi.NewValidationError(fields)
	}
	server.Secrets = secrets
	return server, nil
}

func withoutPrefix(secrets map[string]string, prefix string) map[string]string {
	kept := map[string]string{}
	for key, value := range secrets {
		if !strings.HasPrefix(key, prefix) {
			kept[key] = value
		}
	}
	return kept
}

func replacementSecrets(kind string, entries []Entry, existing map[string]string) map[string]string {
	replaced := map[string]string{}
	for _, entry := range entries {
		if !entry.Secret {
			continue
		}
		if entry.Value != "" {
			replaced[SecretKey(kind, entry.Name)] = entry.Value
		} else if previous, found := existing[SecretKey(kind, entry.Name)]; found {
			replaced[SecretKey(kind, entry.Name)] = previous
		}
	}
	return replaced
}
