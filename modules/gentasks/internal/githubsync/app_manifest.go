package githubsync

import (
	"crypto/rand"
	"encoding/base32"
	"net/url"
	"strings"
)

// Paths the App points at: the webhook on this module's API, and the web app's integration routes.
const (
	WebhookPath             = "/api/gentasks/v1/github/webhook"
	integrationPagePath     = "/tasks/integrations/github"
	integrationCallbackPath = "/tasks/integrations/github/callback"
	suggestedAppNamePrefix  = "Genwork "
	suggestedAppNameSuffix  = 6
)

// AppManifest is the GitHub App manifest format; GitHub reads these exact snake_case keys.
type AppManifest struct {
	Name               string             `json:"name"`
	URL                string             `json:"url"`
	HookAttributes     AppManifestWebhook `json:"hook_attributes"`
	RedirectURL        string             `json:"redirect_url"`
	SetupURL           string             `json:"setup_url"`
	SetupOnUpdate      bool               `json:"setup_on_update"`
	Public             bool               `json:"public"`
	DefaultPermissions map[string]string  `json:"default_permissions"`
	DefaultEvents      []string           `json:"default_events"`
}

type AppManifestWebhook struct {
	URL    string `json:"url"`
	Active bool   `json:"active"`
}

// NewAppManifest describes a private App: a private App can only be installed on the account that owns it, so nobody
// else can point deliveries at this installation (design §2). It reads pull request events and, for bots, writes
// contents and pull requests through short-lived installation tokens.
func NewAppManifest(publicURL string) AppManifest {
	return AppManifest{
		Name:               suggestAppName(),
		URL:                publicURL,
		HookAttributes:     AppManifestWebhook{URL: WebhookURL(publicURL), Active: true},
		RedirectURL:        publicURL + integrationCallbackPath,
		SetupURL:           publicURL + integrationPagePath,
		SetupOnUpdate:      true,
		Public:             false,
		DefaultPermissions: map[string]string{"contents": "write", "pull_requests": "write", "metadata": "read"},
		DefaultEvents:      []string{"pull_request"},
	}
}

func WebhookURL(publicURL string) string {
	return publicURL + WebhookPath
}

// ManifestFormAction is GitHub's App creation page the client posts the manifest to; organization is empty for the
// caller's personal account.
func ManifestFormAction(githubWebURL, organization, state string) string {
	page := githubWebURL + "/settings/apps/new"
	if organization != "" {
		page = githubWebURL + "/organizations/" + url.PathEscape(organization) + "/settings/apps/new"
	}
	return page + "?state=" + url.QueryEscape(state)
}

// suggestAppName adds a random suffix because App names are unique across GitHub; the admin can still edit it there.
func suggestAppName() string {
	random := make([]byte, 4)
	_, _ = rand.Read(random)
	suffix := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random))
	return suggestedAppNamePrefix + suffix[:suggestedAppNameSuffix]
}
