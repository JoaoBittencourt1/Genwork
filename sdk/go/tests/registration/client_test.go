package registration_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/manifest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/registration"
)

func testManifest() manifest.ModuleManifest {
	return manifest.ModuleManifest{
		ID: "gentasks", Version: "0.1.0", Core: ">=1.0.0 <2.0.0", APIPrefix: "/api/gentasks/v1",
		Permissions: []manifest.Permission{{Name: "gentasks.board.read", DefaultRoles: []string{"admin", "member"}}},
	}
}

func newClient(coreURL, secret string) *registration.RegistrationClient {
	return registration.NewRegistrationClient(registration.RegistrationClientConfig{
		CoreURL: coreURL + "/", ModuleSecret: secret, InitialBackoff: time.Millisecond, MaxBackoff: 4 * time.Millisecond,
	})
}

func TestRegisterSendsManifestWithSecret(t *testing.T) {
	core := coretest.NewFakeCore(t)

	registered, err := newClient(core.URL(), coretest.ModuleSecret).Register(context.Background(), testManifest())

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if registered.ID != "gentasks" || registered.Version != "0.1.0" || !registered.Enabled || registered.Token == "" {
		t.Errorf("registered = %+v", registered)
	}
	sent := core.Registrations()
	if len(sent) != 1 || sent[0].Permissions[0].Name != "gentasks.board.read" {
		t.Errorf("registrations = %+v", sent)
	}
}

func TestRegisterReportsRejectionAsPermanent(t *testing.T) {
	core := coretest.NewFakeCore(t)
	core.RejectRegistrations(http.StatusConflict, "incompatible_core_version")

	_, err := newClient(core.URL(), coretest.ModuleSecret).RegisterUntilAccepted(context.Background(), testManifest())

	var rejected *registration.RejectedError
	if !errors.As(err, &rejected) || rejected.Status != http.StatusConflict || rejected.Code != "incompatible_core_version" {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(err, registration.ErrRegistrationRejected) || rejected.Error() == "" {
		t.Errorf("rejection must wrap ErrRegistrationRejected: %v", err)
	}
}

func TestRegisterRejectsWrongSecret(t *testing.T) {
	core := coretest.NewFakeCore(t)

	_, err := newClient(core.URL(), "wrong").Register(context.Background(), testManifest())

	if !errors.Is(err, registration.ErrRegistrationRejected) {
		t.Errorf("error = %v", err)
	}
}

func TestRegisterUntilAcceptedRetriesTransientFailures(t *testing.T) {
	core := coretest.NewFakeCore(t)
	core.FailNextRegistrations(3)

	registered, err := newClient(core.URL(), coretest.ModuleSecret).RegisterUntilAccepted(context.Background(), testManifest())

	if err != nil || registered.ID != "gentasks" {
		t.Fatalf("registered = %+v, err = %v", registered, err)
	}
	if len(core.Registrations()) != 1 {
		t.Errorf("registrations = %d", len(core.Registrations()))
	}
}

func TestRegisterUntilAcceptedStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := newClient("http://127.0.0.1:1", coretest.ModuleSecret).RegisterUntilAccepted(ctx, testManifest())

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v", err)
	}
}
