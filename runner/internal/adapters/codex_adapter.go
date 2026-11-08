package adapters

import (
	"path/filepath"
	"regexp"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const (
	codexProfileMount = "/home/sandbox/.codex"
	codexHomeRelative = GenworkDir + "/codex-home"
	codexAuthFile     = "auth.json"
)

// codexAdapter runs Codex headless: `codex exec --json`, resumed with `codex exec resume`. Its MCP servers live in a
// config.toml of a per-run copy of CODEX_HOME, which holds a copy of the account's auth file; whatever the CLI refreshed
// is copied back when the run ends. Skills are files in the workspace plus a note in the instructions.
type codexAdapter struct{}

func newCodexAdapter() Adapter { return codexAdapter{} }

func (codexAdapter) CLI() string          { return "codex" }
func (codexAdapter) ProfileMount() string { return codexProfileMount }

func (codexAdapter) Prepare(workspace, profileDir string, run api.Run, token api.GenworkToken) error {
	for _, skill := range run.Skills {
		if err := writeSkillFiles(workspace, GenworkDir+"/skills", skill); err != nil {
			return err
		}
	}
	if err := writeFile(workspace, codexHomeRelative+"/config.toml", codexConfigTOML(run, token), privateFileMode); err != nil {
		return err
	}
	if run.Account.Mode == "subscription" && profileDir != "" {
		return copyFileIfExists(filepath.Join(profileDir, codexAuthFile), filepath.Join(workspace, codexHomeRelative, codexAuthFile))
	}
	return nil
}

func (codexAdapter) Invocation(run api.Run, prompt, sessionID string) Invocation {
	fullPrompt := prompt
	if system := SystemPrompt(run, false); system != "" {
		fullPrompt = system + "\n\n" + prompt
	}
	command := []string{"codex", "exec"}
	if sessionID != "" {
		command = append(command, "resume", sessionID)
	}
	command = append(command, "--json", "--model", run.Model, "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check")
	if run.Effort != "" && run.Effort != "default" {
		command = append(command, "-c", "model_reasoning_effort="+run.Effort)
	}
	command = append(command, fullPrompt)
	environment := map[string]string{"CODEX_HOME": sandbox.WorkspaceMount + "/" + codexHomeRelative}
	if run.Account.Mode == "api_key" {
		environment["OPENAI_API_KEY"] = run.Account.APIKey
	}
	return Invocation{Command: command, Env: environment}
}

// codexUserCodePattern matches the one-time code the device flow prints, such as "ABCD-1234".
var codexUserCodePattern = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4,5}\b`)

// SignIn is Codex's device code login: the person opens OpenAI's device page and enters the code; the CLI polls.
func (codexAdapter) SignIn() *SignInSpec {
	return &SignInSpec{
		Command: []string{"codex", "login", "--device-auth"}, Env: map[string]string{"CODEX_HOME": codexProfileMount}, UserCodePattern: codexUserCodePattern,
	}
}

func (codexAdapter) NewParser(secrets []string) Parser { return newCodexParser(secrets...) }

// Finish copies the auth file the CLI may have refreshed back into the account's profile.
func (codexAdapter) Finish(workspace, profileDir string) error {
	if profileDir == "" {
		return nil
	}
	return copyFileIfExists(filepath.Join(workspace, codexHomeRelative, codexAuthFile), filepath.Join(profileDir, codexAuthFile))
}
