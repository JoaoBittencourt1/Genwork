package githubsync

import (
	"fmt"
	"log/slog"
)

const redactedCredential = "[redacted]"

// GithubCredential keeps a secret in clear only in memory: printing, logging, or encoding it shows a placeholder (rule 67).
type GithubCredential struct {
	value []byte
}

func NewGithubCredential(value []byte) GithubCredential {
	return GithubCredential{value: value}
}

func (credential GithubCredential) Reveal() []byte {
	return credential.value
}

func (credential GithubCredential) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte(redactedCredential))
}

func (credential GithubCredential) LogValue() slog.Value {
	return slog.StringValue(redactedCredential)
}

func (credential GithubCredential) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redactedCredential + `"`), nil
}
