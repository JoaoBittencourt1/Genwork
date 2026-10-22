package packaging

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type installScriptValues struct {
	ReleaseURL string
	Version    string
}

// RenderInstallScript fills in the releases page of the repository that publishes this release, as GitHub Actions
// reports it, so no host is written into the template (rule 1).
func RenderInstallScript(scriptTemplate []byte, version string, getenv func(string) string) ([]byte, error) {
	var missing []string
	for _, name := range []string{"GITHUB_SERVER_URL", "GITHUB_REPOSITORY"} {
		if strings.TrimSpace(getenv(name)) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("render install.sh: %s must be set", strings.Join(missing, " and "))
	}
	parsed, err := template.New("install.sh").Option("missingkey=error").Parse(string(scriptTemplate))
	if err != nil {
		return nil, fmt.Errorf("parse install.sh template: %w", err)
	}
	values := installScriptValues{
		ReleaseURL: strings.TrimRight(getenv("GITHUB_SERVER_URL"), "/") + "/" + strings.Trim(getenv("GITHUB_REPOSITORY"), "/") + "/releases",
		Version:    version,
	}
	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, values); err != nil {
		return nil, fmt.Errorf("render install.sh: %w", err)
	}
	return rendered.Bytes(), nil
}
