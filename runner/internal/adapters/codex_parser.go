package adapters

import (
	"encoding/json"
	"strings"
)

// codexParser reads `codex exec --json`: thread.started carries the session, item.completed carries agent messages and
// command executions, and error or turn.failed carry failures.
type codexParser struct {
	secrets  []string
	outcome  Outcome
	lastText string
}

func newCodexParser(secrets ...string) *codexParser {
	return &codexParser{secrets: secrets}
}

type codexLine struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id"`
	Message  string `json:"message"`
	Error    struct {
		Message string `json:"message"`
	} `json:"error"`
	Item struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Command string `json:"command"`
	} `json:"item"`
}

func (parser *codexParser) Feed(line []byte) []Event {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return nil
	}
	var parsed codexLine
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil || parsed.Type == "" {
		parser.lastText = trimmed
		return []Event{{Type: EventAssistantMessage, Payload: map[string]any{"text": Redact(trimmed, parser.secrets...)}}}
	}
	switch parsed.Type {
	case "thread.started":
		parser.outcome.SessionID = parsed.ThreadID
	case "item.completed":
		return parser.item(parsed)
	case "error", "turn.failed":
		message := parsed.Message
		if message == "" {
			message = parsed.Error.Message
		}
		parser.outcome.Failed = true
		parser.outcome.ErrorMessage = Redact(message, parser.secrets...)
		parser.outcome.AuthFailure = LooksLikeAuthFailure(message)
	}
	return nil
}

func (parser *codexParser) item(parsed codexLine) []Event {
	switch parsed.Item.Type {
	case "agent_message":
		parser.lastText = parsed.Item.Text
		return []Event{{Type: EventAssistantMessage, Payload: map[string]any{"text": Redact(parsed.Item.Text, parser.secrets...)}}}
	case "command_execution":
		return []Event{{Type: EventToolUse, Payload: map[string]any{"tool": "shell", "summary": Summarize("shell", map[string]any{"command": parsed.Item.Command}, parser.secrets...)}}}
	}
	return nil
}

func (parser *codexParser) Outcome() Outcome {
	parser.outcome.FinalText = parser.lastText
	return parser.outcome
}
