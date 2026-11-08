package adapters_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/adapters"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
)

func feedFixture(t *testing.T, parser adapters.Parser, name string) []adapters.Event {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()
	var events []adapters.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		events = append(events, parser.Feed(scanner.Bytes())...)
	}
	return events
}

func typesOf(events []adapters.Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func sampleRun(provider, mode string) api.Run {
	return api.Run{
		RunToken: "run-1", Kind: "task", Model: "claude-sonnet-5-5", Effort: "high", MaxTurns: 80, Instructions: "Follow the conventions.",
		Account:       api.ClaimedAccount{Token: "acct-1", Provider: provider, Mode: mode, APIKey: "sk-run-key-1234"},
		GenworkMCPURL: "http://genwork.test/mcp",
		Skills:        []api.Skill{{Name: "go-conventions", Description: "How we write Go", Body: "# Go", Files: []api.SkillFile{{Path: "examples/a.go", Content: "package a"}}}},
		MCPServers: []api.MCPServer{
			{Name: "sentry", Transport: "http", URL: "https://mcp.sentry.dev/mcp", Headers: []api.Entry{{Name: "Authorization", Value: "Bearer sentry-token"}}},
			{Name: "files", Transport: "stdio", Command: "npx", Args: []string{"-y", "files-mcp"}, Env: []api.Entry{{Name: "TOKEN", Value: "env-secret"}}},
		},
	}
}

var sampleToken = api.GenworkToken{AccessToken: "genwork-access-token", MCPURL: "http://genwork.test/mcp"}

func TestClaudeStreamBecomesEventsAndAnOutcome(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	parser := adapter.NewParser([]string{"sk-run-key-1234"})

	events := feedFixture(t, parser, "claude_success.jsonl")

	if got := strings.Join(typesOf(events), ","); got != "assistant_message,tool_use,assistant_message" {
		t.Errorf("events = %s", got)
	}
	if summary := events[1].Payload["summary"]; summary != "Bash: go test ./... -run TestOrders" {
		t.Errorf("tool summary = %q", summary)
	}
	outcome := parser.Outcome()
	if outcome.SessionID != "claude-session-1" || outcome.Failed || !strings.Contains(outcome.FinalText, "pullRequestTitle") {
		t.Errorf("outcome = %+v", outcome)
	}
}

func TestClaudeAuthFailureIsReportedAsAnAccountError(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	parser := adapter.NewParser(nil)

	feedFixture(t, parser, "claude_auth_failure.jsonl")

	outcome := parser.Outcome()
	if !outcome.Failed || !outcome.AuthFailure || !strings.Contains(outcome.ErrorMessage, "Invalid API key") || outcome.SessionID != "claude-session-2" {
		t.Errorf("outcome = %+v", outcome)
	}
}

func TestSecretsNeverReachTheConversation(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	parser := adapter.NewParser([]string{"sk-ant-secret-0001"})

	events := feedFixture(t, parser, "claude_tool_with_secret.jsonl")

	encoded, _ := json.Marshal(events)
	for _, secret := range []string{"abcdef1234567890", "sk-ant-secret-0001"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("a secret reached the events: %s", encoded)
		}
	}
}

func TestGarbageOutputStillProducesAConversationAndNoSession(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	parser := adapter.NewParser(nil)

	events := feedFixture(t, parser, "garbage.txt")

	if len(events) != 2 || events[0].Type != "assistant_message" || events[0].Payload["text"] != "Segmentation fault (core dumped)" {
		t.Errorf("events = %+v", events)
	}
	if outcome := parser.Outcome(); outcome.SessionID != "" || outcome.Failed || outcome.FinalText != "not json at all" {
		t.Errorf("outcome = %+v", outcome)
	}
}

func TestCodexStream(t *testing.T) {
	adapter, _ := adapters.For("openai")
	parser := adapter.NewParser(nil)

	events := feedFixture(t, parser, "codex_success.jsonl")

	if got := strings.Join(typesOf(events), ","); got != "tool_use,assistant_message" {
		t.Errorf("events = %s", got)
	}
	if outcome := parser.Outcome(); outcome.SessionID != "codex-thread-1" || outcome.Failed || !strings.Contains(outcome.FinalText, "GEN-2") {
		t.Errorf("outcome = %+v", outcome)
	}
	failing := adapter.NewParser(nil)
	feedFixture(t, failing, "codex_auth_failure.jsonl")
	if outcome := failing.Outcome(); !outcome.Failed || !outcome.AuthFailure || outcome.SessionID != "codex-thread-2" {
		t.Errorf("failing outcome = %+v", outcome)
	}
}

func TestEachProviderHasAnAdapter(t *testing.T) {
	for provider, cli := range map[string]string{"anthropic": "claude", "openai": "codex", "cursor": "cursor-agent", "xai": "grok"} {
		adapter, err := adapters.For(provider)
		if err != nil || adapter.CLI() != cli {
			t.Errorf("%s: adapter = %v, err = %v", provider, adapter, err)
		}
	}
	if _, err := adapters.For("mystery"); err == nil {
		t.Error("an unknown provider has an adapter")
	}
	if len(adapters.CLIs()) != 4 {
		t.Errorf("CLIs = %v", adapters.CLIs())
	}
}

func TestClaudeInvocationByMode(t *testing.T) {
	adapter, _ := adapters.For("anthropic")

	apiKey := adapter.Invocation(sampleRun("anthropic", "api_key"), "do it", "")
	subscription := adapter.Invocation(sampleRun("anthropic", "subscription"), "do it", "session-9")

	if apiKey.Env["ANTHROPIC_API_KEY"] != "sk-run-key-1234" || apiKey.Profile != nil || apiKey.Env["CLAUDE_CONFIG_DIR"] != "" {
		t.Errorf("api key invocation = %+v", apiKey)
	}
	if subscription.Env["CLAUDE_CONFIG_DIR"] != "/home/sandbox/.claude" || subscription.Profile == nil || subscription.Profile.Mount != "/home/sandbox/.claude" || subscription.Env["ANTHROPIC_API_KEY"] != "" {
		t.Errorf("subscription invocation = %+v", subscription)
	}
	command := strings.Join(subscription.Command, " ")
	for _, want := range []string{"claude -p do it", "--output-format stream-json", "--dangerously-skip-permissions", "--model claude-sonnet-5-5", "--max-turns 80", "--mcp-config /workspace/.genwork/mcp.json", "--resume session-9", "--append-system-prompt Follow the conventions."} {
		if !strings.Contains(command, want) {
			t.Errorf("the command lacks %q: %s", want, command)
		}
	}
	if strings.Contains(strings.Join(apiKey.Command, " "), "--resume") {
		t.Error("a first turn resumes a session")
	}
}

func TestCodexInvocation(t *testing.T) {
	adapter, _ := adapters.For("openai")

	first := adapter.Invocation(sampleRun("openai", "api_key"), "do it", "")
	resumed := adapter.Invocation(sampleRun("openai", "subscription"), "go on", "thread-3")

	firstCommand := strings.Join(first.Command, " ")
	if !strings.HasPrefix(firstCommand, "codex exec --json --model") || !strings.Contains(firstCommand, "model_reasoning_effort=high") ||
		first.Env["OPENAI_API_KEY"] != "sk-run-key-1234" || first.Env["CODEX_HOME"] != "/workspace/.genwork/codex-home" {
		t.Errorf("first = %s %+v", firstCommand, first.Env)
	}
	if !strings.Contains(strings.Join(resumed.Command, " "), "codex exec resume thread-3 --json") || resumed.Env["OPENAI_API_KEY"] != "" {
		t.Errorf("resumed = %v", resumed)
	}
	if !strings.Contains(first.Command[len(first.Command)-1], "go-conventions") {
		t.Error("the skills are not part of the prompt for a CLI without native skills")
	}
}

func TestCursorAndGrokInvocations(t *testing.T) {
	cursor, _ := adapters.For("cursor")
	grok, _ := adapters.For("xai")

	cursorSub := cursor.Invocation(sampleRun("cursor", "subscription"), "do it", "")
	cursorKey := cursor.Invocation(sampleRun("cursor", "api_key"), "do it", "s1")
	grokRun := grok.Invocation(sampleRun("xai", "api_key"), "do it", "")

	if cursorSub.Env["HOME"] != "/home/sandbox/cursor-profile" || cursorSub.Profile == nil || cursorKey.Env["CURSOR_API_KEY"] != "sk-run-key-1234" ||
		!strings.Contains(strings.Join(cursorKey.Command, " "), "--resume s1") {
		t.Errorf("cursor = %+v / %+v", cursorSub, cursorKey)
	}
	if grokRun.Env["GROK_API_KEY"] != "sk-run-key-1234" || grokRun.Profile != nil || grok.ProfileMount() != "" || grokRun.Command[0] != "grok" {
		t.Errorf("grok = %+v", grokRun)
	}
}

func readWorkspaceFile(t *testing.T, workspace, relative string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(workspace, relative))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(content)
}

func TestClaudePrepareWritesSkillsAndMCPConfig(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	workspace := t.TempDir()

	if err := adapter.Prepare(workspace, "", sampleRun("anthropic", "api_key"), sampleToken); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	skill := readWorkspaceFile(t, workspace, ".claude/skills/go-conventions/SKILL.md")
	if !strings.HasPrefix(skill, "---\nname: go-conventions\ndescription: How we write Go\n---") || !strings.Contains(skill, "# Go") {
		t.Errorf("SKILL.md = %s", skill)
	}
	if readWorkspaceFile(t, workspace, ".claude/skills/go-conventions/examples/a.go") != "package a" {
		t.Error("the extra skill file is missing")
	}
	var config struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(readWorkspaceFile(t, workspace, ".genwork/mcp.json")), &config); err != nil {
		t.Fatalf("decode mcp.json: %v", err)
	}
	genwork := config.MCPServers["genwork"]
	if genwork["url"] != "http://genwork.test/mcp" || genwork["headers"].(map[string]any)["Authorization"] != "Bearer genwork-access-token" {
		t.Errorf("genwork server = %v", genwork)
	}
	if config.MCPServers["sentry"]["url"] != "https://mcp.sentry.dev/mcp" || config.MCPServers["files"]["command"] != "npx" || len(config.MCPServers) != 3 {
		t.Errorf("servers = %v", config.MCPServers)
	}
	info, err := os.Stat(filepath.Join(workspace, ".genwork/mcp.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mcp.json mode = %v, err = %v", info.Mode(), err)
	}
}

func TestCodexPrepareWritesConfigTomlAndCopiesTheAuthFile(t *testing.T) {
	adapter, _ := adapters.For("openai")
	workspace, profile := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(profile, "auth.json"), []byte(`{"token":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := adapter.Prepare(workspace, profile, sampleRun("openai", "subscription"), sampleToken); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	config := readWorkspaceFile(t, workspace, ".genwork/codex-home/config.toml")
	for _, want := range []string{"[mcp_servers.genwork]", `url = "http://genwork.test/mcp"`, "Bearer genwork-access-token", "[mcp_servers.sentry]", "[mcp_servers.files]", `command = "npx"`} {
		if !strings.Contains(config, want) {
			t.Errorf("config.toml lacks %q:\n%s", want, config)
		}
	}
	if readWorkspaceFile(t, workspace, ".genwork/codex-home/auth.json") != `{"token":"one"}` {
		t.Error("the auth file was not copied")
	}
	if err := os.WriteFile(filepath.Join(workspace, ".genwork/codex-home/auth.json"), []byte(`{"token":"refreshed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Finish(workspace, profile); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if content, _ := os.ReadFile(filepath.Join(profile, "auth.json")); string(content) != `{"token":"refreshed"}` {
		t.Errorf("profile auth file = %s", content)
	}
}

func TestCursorAndGrokPrepare(t *testing.T) {
	for provider, path := range map[string]string{"cursor": ".cursor/mcp.json", "xai": ".grok/mcp.json"} {
		adapter, _ := adapters.For(provider)
		workspace := t.TempDir()

		if err := adapter.Prepare(workspace, "", sampleRun(provider, "api_key"), sampleToken); err != nil {
			t.Fatalf("%s prepare: %v", provider, err)
		}

		if !strings.Contains(readWorkspaceFile(t, workspace, path), "genwork-access-token") {
			t.Errorf("%s: the MCP config lacks the token", provider)
		}
		if !strings.Contains(readWorkspaceFile(t, workspace, ".genwork/skills/go-conventions/SKILL.md"), "# Go") {
			t.Errorf("%s: the skill is missing", provider)
		}
	}
	cursor, _ := adapters.For("cursor")
	workspace := t.TempDir()
	if err := cursor.Prepare(workspace, "", sampleRun("cursor", "api_key"), sampleToken); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readWorkspaceFile(t, workspace, ".cursor/rules/go-conventions.mdc"), "alwaysApply: false") {
		t.Error("the cursor rule is missing")
	}
}

func TestPrepareRefusesSkillFilesOutsideTheWorkspace(t *testing.T) {
	adapter, _ := adapters.For("anthropic")
	run := sampleRun("anthropic", "api_key")
	run.Skills[0].Files = []api.SkillFile{{Path: "../../escape.txt", Content: "x"}}

	if err := adapter.Prepare(t.TempDir(), "", run, sampleToken); err == nil {
		t.Error("a skill file escaped the workspace")
	}
}

func TestRedactAndSummarize(t *testing.T) {
	if got := adapters.Redact("key sk-live-abcdefgh1234 and ghp_abcdefghij1234 and my-own-secret", "my-own-secret"); strings.Contains(got, "sk-live") || strings.Contains(got, "ghp_") || strings.Contains(got, "my-own-secret") {
		t.Errorf("redacted = %s", got)
	}
	long := adapters.Summarize("Bash", map[string]any{"command": strings.Repeat("x", 300)})
	if len([]rune(long)) > 140 || !strings.HasSuffix(long, "…") {
		t.Errorf("summary length = %d", len([]rune(long)))
	}
	if adapters.Summarize("Task", map[string]any{"x": 1}) != "Task" {
		t.Error("a tool without a telling input shows more than its name")
	}
	if !adapters.LooksLikeAuthFailure("401 Unauthorized") || adapters.LooksLikeAuthFailure("rate limited") {
		t.Error("auth failure detection is wrong")
	}
	run := sampleRun("openai", "api_key")
	if prompt := adapters.SystemPrompt(run, false); !strings.Contains(prompt, ".genwork/skills/go-conventions/SKILL.md") || !strings.Contains(prompt, "Follow the conventions.") {
		t.Errorf("system prompt = %s", prompt)
	}
	if prompt := adapters.SystemPrompt(run, true); strings.Contains(prompt, "skills") {
		t.Errorf("native skills were listed in the prompt: %s", prompt)
	}
}

func TestSubscriptionProvidersSignInThroughTheirOwnCLI(t *testing.T) {
	cases := map[string]struct {
		command     string
		profileEnv  string
		expectsCode bool
	}{
		"anthropic": {"claude auth login", "CLAUDE_CONFIG_DIR", true},
		"openai":    {"codex login --device-auth", "CODEX_HOME", false},
		"cursor":    {"cursor-agent login", "HOME", false},
	}
	for provider, want := range cases {
		adapter, _ := adapters.For(provider)
		signIn := adapter.SignIn()
		if signIn == nil {
			t.Fatalf("%s has no sign-in", provider)
		}
		if strings.Join(signIn.Command, " ") != want.command || signIn.ExpectsCode != want.expectsCode {
			t.Errorf("%s sign-in = %+v", provider, signIn)
		}
		if signIn.Env[want.profileEnv] != adapter.ProfileMount() {
			t.Errorf("%s sign-in does not write the mounted profile: %v", provider, signIn.Env)
		}
	}
	grok, _ := adapters.For("xai")
	if grok.SignIn() != nil {
		t.Error("xai offers a subscription sign-in")
	}
}

func TestCodexDeviceCodePatternMatchesItsOneTimeCode(t *testing.T) {
	codex, _ := adapters.For("openai")

	if got := codex.SignIn().UserCodePattern.FindString("Enter this one-time code: ABCD-12345 (expires in 15 minutes)"); got != "ABCD-12345" {
		t.Errorf("code = %q", got)
	}
}

func TestTerminalControlSequencesNeverBecomeBotMessages(t *testing.T) {
	for _, provider := range []string{"anthropic", "openai"} {
		adapter, _ := adapters.For(provider)
		parser := adapter.NewParser(nil)

		events := append(parser.Feed([]byte("\x1b[?25h")), parser.Feed([]byte("\x1b[1mDone\x1b[0m"))...)

		if len(events) != 1 || events[0].Payload["text"] != "Done" {
			t.Errorf("%s events = %+v", provider, events)
		}
	}
}
