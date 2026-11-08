package adapters

import "github.com/JoaoBittencourt1/Genwork/runner/internal/api"

// grokAdapter runs the Grok CLI in headless prompt mode. xAI has no subscription sign-in for a CLI, so it uses an API key
// only. Its MCP servers come from .grok/mcp.json in the workspace, and its skills are files plus a note in the
// instructions. Its output is read like the stream JSON of the other CLIs, and plain lines still reach the conversation.
type grokAdapter struct{}

func newGrokAdapter() Adapter { return grokAdapter{} }

func (grokAdapter) CLI() string          { return "grok" }
func (grokAdapter) ProfileMount() string { return "" }

func (grokAdapter) Prepare(workspace, _ string, run api.Run, token api.GenworkToken) error {
	for _, skill := range run.Skills {
		if err := writeSkillFiles(workspace, GenworkDir+"/skills", skill); err != nil {
			return err
		}
	}
	document, err := mcpJSON(run, token)
	if err != nil {
		return err
	}
	return writeFile(workspace, ".grok/mcp.json", document, privateFileMode)
}

func (grokAdapter) Invocation(run api.Run, prompt, sessionID string) Invocation {
	fullPrompt := prompt
	if system := SystemPrompt(run, false); system != "" {
		fullPrompt = system + "\n\n" + prompt
	}
	command := []string{"grok", "-p", fullPrompt, "--output-format", "stream-json", "--model", run.Model}
	if sessionID != "" {
		command = append(command, "--resume", sessionID)
	}
	return Invocation{Command: command, Env: map[string]string{"GROK_API_KEY": run.Account.APIKey}}
}

// SignIn is nil: xAI has no subscription sign-in for its CLI.
func (grokAdapter) SignIn() *SignInSpec { return nil }

func (grokAdapter) NewParser(secrets []string) Parser { return newStreamParser(secrets...) }

func (grokAdapter) Finish(string, string) error { return nil }
