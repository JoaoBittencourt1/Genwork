package runs

import (
	"encoding/json"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

const MaxMessageLength = 8000

type ConversationKind string

const (
	ConversationKindTask      ConversationKind = "task"
	ConversationKindAdminChat ConversationKind = "admin_chat"
)

type EntryType string

const (
	EntryHumanMessage     EntryType = "human_message"
	EntryStatus           EntryType = "status"
	EntryAssistantMessage EntryType = "assistant_message"
	EntryToolUse          EntryType = "tool_use"
	EntryResult           EntryType = "result"
	EntryError            EntryType = "error"
)

// IsRunnerEntry says which entry types a runner may post; people's messages come only through the conversation API.
func (entryType EntryType) IsRunnerEntry() bool {
	switch entryType {
	case EntryStatus, EntryAssistantMessage, EntryToolUse, EntryResult, EntryError:
		return true
	}
	return false
}

var (
	ErrConversationNotFound = httpapi.NewAPIError(http.StatusNotFound, "conversation_not_found")
	ErrMessageInvalid       = httpapi.NewValidationError(map[string]string{"message": "invalid"})
)

func IsValidMessage(message string) bool {
	length := utf8.RuneCountInString(message)
	return length > 0 && length <= MaxMessageLength
}

type Conversation struct {
	ID               int64
	Token            string
	Kind             ConversationKind
	TaskToken        *string
	OwnerUserToken   *string
	Title            *string
	BotID            *int64
	DelegatedSession *string
	DelegatedRefresh []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Entry is one line of a conversation. Sequence orders the conversation; RunSequence orders a run's own events and makes
// a retried post idempotent.
type Entry struct {
	ID          int64
	Token       string
	Sequence    int64
	RunToken    *string
	RunSequence *int64
	Type        EntryType
	AuthorToken *string
	Payload     []byte
	DeliveredAt *time.Time
	CreatedAt   time.Time
}

// MessageText reads the text of a human message payload.
func MessageText(payload []byte) string {
	var message struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return ""
	}
	return message.Text
}
