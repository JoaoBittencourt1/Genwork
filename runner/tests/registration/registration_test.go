package registration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/config"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/registration"
)

func TestRegisterStoresTheSecretOwnerOnlyAndKeepsTheGithubToken(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	home := t.TempDir()
	if err := config.Save(home, config.Stored{GithubToken: "ghp-existing"}); err != nil {
		t.Fatal(err)
	}

	err := registration.Register(context.Background(), home, fake.URL()+"/", " registration-token ", nil)

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	stored, _ := config.LoadStored(home)
	if stored.RunnerSecret != genagentstest.Secret || stored.RunnerToken != "runner-token" || stored.InstanceURL != fake.URL() || stored.GithubToken != "ghp-existing" {
		t.Errorf("stored = %+v", stored)
	}
	info, err := os.Stat(filepath.Join(home, "runner.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, err = %v", info.Mode(), err)
	}
}

func TestReusedRegistrationTokenFails(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	home := t.TempDir()
	if err := registration.Register(context.Background(), home, fake.URL(), "registration-token", nil); err != nil {
		t.Fatalf("first: %v", err)
	}

	err := registration.Register(context.Background(), t.TempDir(), fake.URL(), "registration-token", nil)

	var statusErr *api.StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "invalid_registration_token" {
		t.Errorf("error = %v", err)
	}
}

func TestRegisterNeedsBothInputs(t *testing.T) {
	for _, inputs := range [][2]string{{"", "token"}, {"https://x.io", ""}, {" ", " "}} {
		if err := registration.Register(context.Background(), t.TempDir(), inputs[0], inputs[1], nil); !errors.Is(err, registration.ErrMissingInput) {
			t.Errorf("%v: error = %v", inputs, err)
		}
	}
}
