package adapters

import (
	"encoding/json"
	"strings"
)

// streamParser reads the stream-json output Claude Code and the Cursor agent share: one JSON object per line, with a
// system init carrying the session, assistant messages holding text and tool uses, and a final result. A line that is
// not JSON is kept as assistant text, so a CLI that prints plain text still produces a conversation.
type streamParser struct {
	secrets           []string
	outcome           Outcome
	lastAssistantText string
}

func newStreamParser(secrets ...string) *streamParser {
	return &streamParser{secrets: secrets}
}

type streamLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	Message   struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

func (parser *streamParser) Feed(line []byte) []Event {
	trimmed := strings.TrimSpace(StripTerminalEscapes(string(line)))
	if trimmed == "" {
		return nil
	}
	var parsed streamLine
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil || parsed.Type == "" {
		return parser.plainText(trimmed)
	}
	if parsed.SessionID != "" {
		parser.outcome.SessionID = parsed.SessionID
	}
	switch parsed.Type {
	case "assistant":
		return parser.assistant(parsed)
	case "result":
		return parser.result(parsed)
	}
	return nil
}

func (parser *streamParser) plainText(text string) []Event {
	parser.lastAssistantText = text
	return []Event{{Type: EventAssistantMessage, Payload: map[string]any{"text": Redact(text, parser.secrets...)}}}
}

func (parser *streamParser) assistant(parsed streamLine) []Event {
	var events []Event
	for _, block := range parsed.Message.Content {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			parser.lastAssistantText = block.Text
			events = append(events, Event{Type: EventAssistantMessage, Payload: map[string]any{"text": Redact(block.Text, parser.secrets...)}})
		case "tool_use":
			events = append(events, Event{Type: EventToolUse, Payload: map[string]any{"tool": block.Name, "summary": Summarize(block.Name, block.Input, parser.secrets...)}})
		}
	}
	return events
}

func (parser *streamParser) result(parsed streamLine) []Event {
	text := parsed.Result
	if strings.TrimSpace(text) == "" {
		text = parser.lastAssistantText
	}
	parser.outcome.FinalText = text
	if parsed.IsError || strings.HasPrefix(parsed.Subtype, "error") {
		parser.outcome.Failed = true
		parser.outcome.ErrorMessage = Redact(text, parser.secrets...)
		if parser.outcome.ErrorMessage == "" {
			parser.outcome.ErrorMessage = parsed.Subtype
		}
		parser.outcome.AuthFailure = LooksLikeAuthFailure(text)
	}
	return nil
}

func (parser *streamParser) Outcome() Outcome {
	if parser.outcome.FinalText == "" {
		parser.outcome.FinalText = parser.lastAssistantText
	}
	return parser.outcome
}
