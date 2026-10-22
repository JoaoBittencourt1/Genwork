package commands

import (
	"strings"
	"testing"
)

func TestVersionPrintsTheBuildVersionOutsideAnyWorkspace(t *testing.T) {
	workspace := Workspace{Root: t.TempDir()}

	err, output := execute(t, workspace, &fakeRunner{}, &fakePrompter{}, "version")

	if err != nil || strings.TrimSpace(output) != Version {
		t.Errorf("error = %v, output = %q", err, output)
	}
}

func TestBuildWithoutLdflagsReportsTheDevelopmentVersion(t *testing.T) {
	if Version != "0.0.0-dev" {
		t.Errorf("Version = %q", Version)
	}
}
