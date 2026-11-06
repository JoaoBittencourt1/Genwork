package runs

import (
	"encoding/json"
	"time"
)

// EntryResponse is a conversation entry as clients read it; payloads carry message keys the web app localizes.
type EntryResponse struct {
	Sequence    int64           `json:"sequence"`
	Type        EntryType       `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	AuthorToken *string         `json:"authorToken"`
	RunToken    *string         `json:"runToken"`
	CreatedAt   time.Time       `json:"createdAt"`
}

func NewEntryResponses(entries []Entry) []EntryResponse {
	responses := make([]EntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, EntryResponse{
			Sequence: entry.Sequence, Type: entry.Type, Payload: entry.Payload, AuthorToken: entry.AuthorToken, RunToken: entry.RunToken, CreatedAt: entry.CreatedAt,
		})
	}
	return responses
}

type ActiveRunResponse struct {
	Token   string   `json:"token"`
	Kind    RunKind  `json:"kind"`
	State   RunState `json:"state"`
	BotName string   `json:"botName"`
}
