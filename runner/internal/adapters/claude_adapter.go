package adapters

import (
	"strconv"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const (
	claudeProfileMount = "/home/sandbox/.claude"
	claudeMCPConfig    = sandbox.WorkspaceMount + "/" + GenworkDir + "/mcp.json"
)

// claudeAdapter runs Claude Code headless: `claude -p` with stream JSON output, resumed with --resume. Skills are native
// project skills in the workspace, and MCP servers come from a --mcp-config file.
type claudeAdapter struct{}

func newClaudeAdapter() Adapter { return claudeAdapter{} }

func (claudeAdapter) CLI() string          { return "claude" }
func (claudeAdapter) ProfileMount() string { return claudeProfileMount }

func (claudeAdapter) Prepare(workspace, _ string, run api.Run, token api.GenworkToken) error {
	for _, skill := range run.Skills {
		if err := writeSkillFiles(workspace, ".claude/skills", skill); err != nil {
			return err
		}
	}
	document, err := mcpJSON(run, token)
	if err != nil {
		return err
	}
	return writeFile(workspace, GenworkDir+"/mcp.json", document, privateFileMode)
}

func (claudeAdapter) Invocation(run api.Run, prompt, sessionID string) Invocation {
	command := []string{"claude", "-p", prompt, "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions",
		"--model", run.Model, "--max-turns", strconv.Itoa(run.MaxTurns), "--mcp-config", claudeMCPConfig}
	if system := SystemPrompt(run, true); system != "" {
		command = append(command, "--append-system-prompt", system)
	}
	if sessionID != "" {
		command = append(command, "--resume", sessionID)
	}
	environment := map[string]string{}
	invocation := Invocation{Command: command, Env: environment}
	if run.Account.Mode == "api_key" {
		environment["ANTHROPIC_API_KEY"] = run.Account.APIKey
	} else {
		environment["CLAUDE_CONFIG_DIR"] = claudeProfileMount
		invocation.Profile = &sandbox.Profile{Mount: claudeProfileMount}
	}
	return invocation
}

func (claudeAdapter) LoginCommand() []string { return []string{"claude", "/login"} }

func (claudeAdapter) LoginEnv() map[string]string {
	return map[string]string{"CLAUDE_CONFIG_DIR": claudeProfileMount}
}

func (claudeAdapter) NewParser(secrets []string) Parser { return newStreamParser(secrets...) }

func (claudeAdapter) Finish(string, string) error { return nil }
