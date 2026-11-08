package adapters

import (
	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const cursorProfileMount = "/home/sandbox/cursor-profile"

// cursorAdapter runs the Cursor agent CLI headless with stream JSON output. Its MCP servers come from .cursor/mcp.json in
// the workspace, its skills are rules in .cursor/rules plus a note in the instructions, and its sign-in profile is the
// HOME it runs with.
type cursorAdapter struct{}

func newCursorAdapter() Adapter { return cursorAdapter{} }

func (cursorAdapter) CLI() string          { return "cursor-agent" }
func (cursorAdapter) ProfileMount() string { return cursorProfileMount }

func (cursorAdapter) Prepare(workspace, _ string, run api.Run, token api.GenworkToken) error {
	for _, skill := range run.Skills {
		if err := writeSkillFiles(workspace, GenworkDir+"/skills", skill); err != nil {
			return err
		}
		rule := "---\ndescription: " + oneLine(skill.Description) + "\nalwaysApply: false\n---\n\n" + skill.Body + "\n"
		if err := writeFile(workspace, ".cursor/rules/"+skill.Name+".mdc", rule, privateFileMode); err != nil {
			return err
		}
	}
	document, err := mcpJSON(run, token)
	if err != nil {
		return err
	}
	return writeFile(workspace, ".cursor/mcp.json", document, privateFileMode)
}

func (cursorAdapter) Invocation(run api.Run, prompt, sessionID string) Invocation {
	fullPrompt := prompt
	if system := SystemPrompt(run, false); system != "" {
		fullPrompt = system + "\n\n" + prompt
	}
	command := []string{"cursor-agent", "-p", fullPrompt, "--output-format", "stream-json", "--force", "--model", run.Model, "--approve-mcps"}
	if sessionID != "" {
		command = append(command, "--resume", sessionID)
	}
	environment := map[string]string{}
	invocation := Invocation{Command: command, Env: environment}
	if run.Account.Mode == "api_key" {
		environment["CURSOR_API_KEY"] = run.Account.APIKey
	} else {
		environment["HOME"] = cursorProfileMount
		invocation.Profile = &sandbox.Profile{Mount: cursorProfileMount}
	}
	return invocation
}

// SignIn is the Cursor agent's browser login: it prints the URL instead of opening a browser and polls until the person
// authorizes.
func (cursorAdapter) SignIn() *SignInSpec {
	return &SignInSpec{Command: []string{"cursor-agent", "login"}, Env: map[string]string{"HOME": cursorProfileMount, "NO_OPEN_BROWSER": "1"}}
}

func (cursorAdapter) NewParser(secrets []string) Parser { return newStreamParser(secrets...) }

func (cursorAdapter) Finish(string, string) error { return nil }
