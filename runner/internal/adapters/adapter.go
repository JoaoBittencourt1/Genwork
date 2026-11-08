// Package adapters is the only place that knows each provider CLI: its command line, where it reads skills and MCP
// configuration, and how its output maps to run events. An adapter builds commands; the sandbox launcher wraps them in a
// container, so an adapter never talks to the container engine (rule 9).
package adapters

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandbox"
)

const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
	ProviderCursor    = "cursor"
	ProviderXAI       = "xai"

	// GenworkDir holds what the runner writes into a workspace for the CLI: configuration, never credentials of the host.
	GenworkDir     = ".genwork"
	maxSummaryRune = 140
)

// Event types a run reports.
const (
	EventStatus           = "status"
	EventAssistantMessage = "assistant_message"
	EventToolUse          = "tool_use"
	EventResult           = "result"
	EventError            = "error"
)

type Event struct {
	Type    string
	Payload map[string]any
}

// Outcome is what a finished turn left: the CLI session to resume, the last thing the model said, and whether the
// provider refused the credentials.
type Outcome struct {
	SessionID    string
	FinalText    string
	Failed       bool
	ErrorMessage string
	AuthFailure  bool
}

// Parser turns one turn's output into events, line by line.
type Parser interface {
	Feed(line []byte) []Event
	Outcome() Outcome
}

// Invocation is one CLI call: the command and environment inside the sandbox, and where the account's profile mounts.
type Invocation struct {
	Command []string
	Env     map[string]string
	Profile *sandbox.Profile
}

// SignInSpec is how a CLI signs a subscription account in. The runner starts Command with Env (which points the CLI's
// profile at ProfileMount) in a terminal, relays the URL it prints, and types the code a person pastes when ExpectsCode
// is set. UserCodePattern finds the one-time code a device flow asks the person to enter on the provider's page.
type SignInSpec struct {
	Command         []string
	Env             map[string]string
	ExpectsCode     bool
	UserCodePattern *regexp.Regexp
}

// Adapter is one provider's CLI.
type Adapter interface {
	// CLI is the tool name the sandbox image reports for this provider.
	CLI() string
	// ProfileMount is where the CLI reads its sign-in profile inside the sandbox.
	ProfileMount() string
	// Prepare writes the run's skills and MCP configuration into the workspace. It runs again whenever the Genwork token
	// is renewed, because the sandbox cannot call the runner.
	Prepare(workspace, profileDir string, run api.Run, token api.GenworkToken) error
	// Invocation builds the call for a turn; sessionID is empty for the first one.
	Invocation(run api.Run, prompt, sessionID string) Invocation
	NewParser(secrets []string) Parser
	// SignIn is the CLI's own sign-in for a subscription account; nil when the provider has none.
	SignIn() *SignInSpec
	// Finish copies back what the CLI changed outside the profile mount, if anything.
	Finish(workspace, profileDir string) error
}

// For returns the adapter of a provider.
func For(provider string) (Adapter, error) {
	switch provider {
	case ProviderAnthropic:
		return newClaudeAdapter(), nil
	case ProviderOpenAI:
		return newCodexAdapter(), nil
	case ProviderCursor:
		return newCursorAdapter(), nil
	case ProviderXAI:
		return newGrokAdapter(), nil
	}
	return nil, fmt.Errorf("no adapter for provider %q", provider)
}

// CLIs lists the tool name of every adapter, the set a sandbox image must provide.
func CLIs() []string {
	return []string{"claude", "codex", "cursor-agent", "grok"}
}

// SystemPrompt is the bot's instructions plus the skills a CLI without native skill support has to be told about.
func SystemPrompt(run api.Run, nativeSkills bool) string {
	parts := []string{}
	if strings.TrimSpace(run.Instructions) != "" {
		parts = append(parts, run.Instructions)
	}
	if !nativeSkills && len(run.Skills) > 0 {
		var skills strings.Builder
		skills.WriteString("You have these skills. Read the SKILL.md of the one that applies before you start:\n")
		for _, skill := range run.Skills {
			fmt.Fprintf(&skills, "- %s: %s (%s/skills/%s/SKILL.md)\n", skill.Name, skill.Description, GenworkDir, skill.Name)
		}
		parts = append(parts, skills.String())
	}
	return strings.Join(parts, "\n\n")
}

var secretPattern = regexp.MustCompile(`(?i)(sk-[a-z0-9_-]{8,}|ghp_[a-z0-9]{10,}|github_pat_[a-z0-9_]{10,}|bearer\s+[a-z0-9._-]{8,}|eyJ[a-z0-9_-]{10,}\.[a-z0-9_-]{5,}\.[a-z0-9_-]{5,})`)

// Redact hides credentials in text meant for the conversation: the run's own secrets and anything shaped like a key.
func Redact(text string, secrets ...string) string {
	for _, secret := range secrets {
		if len(secret) >= 6 {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return secretPattern.ReplaceAllString(text, "[redacted]")
}

// Summarize is the one line a tool call shows in the conversation: its name and the most telling input, never more.
func Summarize(tool string, input map[string]any, secrets ...string) string {
	for _, key := range []string{"command", "file_path", "path", "pattern", "url", "query", "description"} {
		if value, found := input[key].(string); found && strings.TrimSpace(value) != "" {
			return truncate(Redact(tool+": "+firstLine(value), secrets...), maxSummaryRune)
		}
	}
	return tool
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

// authFailurePattern recognizes the ways the provider CLIs say the credentials were refused.
var authFailurePattern = regexp.MustCompile(`(?i)(invalid (x-)?api[ -]?key|authentication[_ ](error|failed)|unauthorized|please run /login|not logged in|incorrect api key|401)`)

// LooksLikeAuthFailure tells whether an error message is the provider refusing the account's credentials.
func LooksLikeAuthFailure(message string) bool {
	return authFailurePattern.MatchString(message)
}

// terminalEscapePattern matches the control sequences a CLI running in a terminal prints: colors, cursor moves (such as
// the "show cursor" it prints on exit), and hyperlinks.
var terminalEscapePattern = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?<>=]*[ -/]*[@-~]|\x1b[()][A-Za-z0-9]|\x1b[=>78DEHMc]`)

// StripTerminalEscapes removes terminal control sequences, leaving the text a person would read.
func StripTerminalEscapes(text string) string {
	return terminalEscapePattern.ReplaceAllString(text, "")
}
