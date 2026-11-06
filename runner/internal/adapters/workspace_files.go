package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
)

const (
	privateFileMode    = 0o600
	workspaceDirectory = 0o755
	genworkServerName  = "genwork"
	bearerPrefix       = "Bearer "
)

// writeFile writes below a workspace and refuses any path that leaves it.
func writeFile(workspace, relative, content string, mode os.FileMode) error {
	target := filepath.Join(workspace, filepath.FromSlash(relative))
	root := filepath.Clean(workspace) + string(filepath.Separator)
	if !strings.HasPrefix(target+string(filepath.Separator), root) {
		return fmt.Errorf("refusing to write %q outside the workspace", relative)
	}
	if err := os.MkdirAll(filepath.Dir(target), workspaceDirectory); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(relative), err)
	}
	return os.WriteFile(target, []byte(content), mode)
}

// writeSkillFiles lays one skill out as a directory with its SKILL.md and extra files below base.
func writeSkillFiles(workspace, base string, skill api.Skill) error {
	directory := base + "/" + skill.Name
	document := "---\nname: " + skill.Name + "\ndescription: " + oneLine(skill.Description) + "\n---\n\n" + skill.Body + "\n"
	if err := writeFile(workspace, directory+"/SKILL.md", document, privateFileMode); err != nil {
		return err
	}
	for _, file := range skill.Files {
		if !isSafeRelativePath(file.Path) {
			return fmt.Errorf("refusing skill file %q: it must stay inside the skill directory", file.Path)
		}
		if err := writeFile(workspace, directory+"/"+file.Path, file.Content, privateFileMode); err != nil {
			return err
		}
	}
	return nil
}

// isSafeRelativePath accepts only clean relative paths without parent references, whatever the server sent.
func isSafeRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	return cleaned == path && cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// mcpJSON builds the {"mcpServers": ...} document Claude Code, the Cursor agent, and the Grok CLI read: the Genwork MCP
// with the run's token, then the bot's own servers.
func mcpJSON(run api.Run, token api.GenworkToken) (string, error) {
	servers := map[string]any{
		genworkServerName: map[string]any{"type": "http", "url": mcpURL(run, token), "headers": map[string]string{"Authorization": bearerPrefix + token.AccessToken}},
	}
	for _, server := range run.MCPServers {
		if server.Transport == "http" {
			headers := map[string]string{}
			for _, header := range server.Headers {
				headers[header.Name] = header.Value
			}
			servers[server.Name] = map[string]any{"type": "http", "url": server.URL, "headers": headers}
			continue
		}
		environment := map[string]string{}
		for _, variable := range server.Env {
			environment[variable.Name] = variable.Value
		}
		servers[server.Name] = map[string]any{"command": server.Command, "args": server.Args, "env": environment}
	}
	encoded, err := json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	return string(encoded), err
}

func mcpURL(run api.Run, token api.GenworkToken) string {
	if token.MCPURL != "" {
		return token.MCPURL
	}
	return run.GenworkMCPURL
}

// codexConfigTOML builds the config.toml Codex reads, with the Genwork MCP first and then the bot's servers.
func codexConfigTOML(run api.Run, token api.GenworkToken) string {
	var document strings.Builder
	fmt.Fprintf(&document, "[mcp_servers.%s]\nurl = %q\nhttp_headers = { Authorization = %q }\n", genworkServerName, mcpURL(run, token), bearerPrefix+token.AccessToken)
	for _, server := range run.MCPServers {
		name := tomlKey(server.Name)
		if server.Transport == "http" {
			fmt.Fprintf(&document, "\n[mcp_servers.%s]\nurl = %q\n", name, server.URL)
			if len(server.Headers) > 0 {
				fmt.Fprintf(&document, "http_headers = { %s }\n", tomlPairs(server.Headers))
			}
			continue
		}
		fmt.Fprintf(&document, "\n[mcp_servers.%s]\ncommand = %q\nargs = %s\n", name, server.Command, tomlStrings(server.Args))
		if len(server.Env) > 0 {
			fmt.Fprintf(&document, "env = { %s }\n", tomlPairs(server.Env))
		}
	}
	return document.String()
}

func tomlKey(name string) string {
	var key strings.Builder
	for _, character := range name {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			key.WriteRune(character)
		} else {
			key.WriteRune('_')
		}
	}
	return key.String()
}

func tomlPairs(entries []api.Entry) string {
	sorted := append([]api.Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	pairs := make([]string, 0, len(sorted))
	for _, entry := range sorted {
		pairs = append(pairs, fmt.Sprintf("%q = %q", entry.Name, entry.Value))
	}
	return strings.Join(pairs, ", ")
}

func tomlStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// copyFileIfExists copies a small file, creating the destination directory; a missing source is not an error.
func copyFileIfExists(from, to string) error {
	content, err := os.ReadFile(from)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	return os.WriteFile(to, content, privateFileMode)
}
