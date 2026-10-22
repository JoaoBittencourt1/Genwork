package githubsync

import (
	"net/http"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/httpapi"
)

var (
	ErrGithubAlreadyConnected = httpapi.NewAPIError(http.StatusConflict, "github_already_connected")
	ErrGithubNotConnected     = httpapi.NewAPIError(http.StatusNotFound, "github_not_connected")
)

type ConnectionState string

const (
	ConnectionDisconnected ConnectionState = "disconnected"
	ConnectionConnected    ConnectionState = "connected"
	// ConnectionUnreadable: an App is stored but its credentials no longer decrypt, e.g. after GENTASKS_ENCRYPTION_KEY changed.
	ConnectionUnreadable ConnectionState = "unreadable"
)

// GithubApp is the App created through the manifest flow; its credentials are only ever stored encrypted (design §3).
type GithubApp struct {
	ID                      int64
	Token                   string
	GithubAppID             int64
	Slug                    string
	OwnerLogin              string
	AppURL                  string
	WebhookSecretCiphertext []byte
	PrivateKeyCiphertext    []byte
	ConnectedBy             string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}
