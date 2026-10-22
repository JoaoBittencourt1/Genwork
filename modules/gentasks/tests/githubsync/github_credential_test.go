package githubsync_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsync"
)

func TestGithubCredentialNeverPrintsItsValue(t *testing.T) {
	credential := githubsync.NewGithubCredential([]byte("super-secret"))
	wrapped := struct{ Secret githubsync.GithubCredential }{credential}

	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
		if printed := fmt.Sprintf(format, credential) + fmt.Sprintf(format, wrapped); strings.Contains(printed, "super-secret") ||
			strings.Contains(printed, fmt.Sprintf("%x", "super-secret")) {
			t.Errorf("%s printed %q", format, printed)
		}
	}
	encoded, _ := json.Marshal(wrapped)
	if strings.Contains(string(encoded), "super-secret") {
		t.Errorf("json = %s", encoded)
	}
	var logged bytes.Buffer
	slog.New(slog.NewTextHandler(&logged, nil)).Info("connected", "secret", credential)
	if strings.Contains(logged.String(), "super-secret") {
		t.Errorf("log = %s", logged.String())
	}
	if string(credential.Reveal()) != "super-secret" {
		t.Error("Reveal must return the value")
	}
}
