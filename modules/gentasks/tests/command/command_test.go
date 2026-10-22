package command_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/command"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/githubsynctest"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/modules/gentasks/internal/platform/dbtest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/registration"
)

func envFrom(values map[string]string) platform.Getenv {
	return func(name string) string { return values[name] }
}

func freeLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func serveEnv(t *testing.T, core *coretest.FakeCore, address string) platform.Getenv {
	return envFrom(map[string]string{
		"GENTASKS_DATABASE_URL":   dbtest.DatabaseURL(t),
		"GENTASKS_HTTP_ADDR":      address,
		"CORE_INTERNAL_URL":       core.URL(),
		"GENWORK_MODULE_SECRET":   coretest.ModuleSecret,
		"GENWORK_PUBLIC_URL":      coretest.Issuer,
		"GENTASKS_ENCRYPTION_KEY": base64.RawURLEncoding.EncodeToString(githubsynctest.EncryptionKey),
		"GITHUB_WEB_URL":          githubsynctest.UnroutableGithubWebURL,
		"GITHUB_API_URL":          githubsynctest.UnroutableGithubAPIURL,
	})
}

func waitForHealth(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + address + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("gentasks did not become healthy")
}

func TestBuildWithoutLdflagsReportsTheDevelopmentVersion(t *testing.T) {
	if command.Version != "0.0.0-dev" {
		t.Errorf("Version = %q", command.Version)
	}
}

func TestRunRejectsUnknownCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"launch"}} {
		if err := command.Run(context.Background(), args, envFrom(nil)); err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("%v: error = %v", args, err)
		}
	}
}

func TestMigrateCommand(t *testing.T) {
	schema := dbtest.NewSchemaName(t)
	env := envFrom(map[string]string{"GENTASKS_DATABASE_URL": dbtest.DatabaseURL(t)})

	if err := command.RunMigrate(context.Background(), env, schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := command.RunMigrate(context.Background(), envFrom(nil), schema); err == nil {
		t.Error("migrate without a database URL must fail")
	}
	if err := command.Run(context.Background(), []string{"migrate"}, envFrom(nil)); err == nil {
		t.Error("migrate through Run without a database URL must fail")
	}
}

func TestServeFailsFastOnInvalidConfiguration(t *testing.T) {
	if err := command.Run(context.Background(), []string{"serve"}, envFrom(nil)); err == nil || !strings.Contains(err.Error(), "GENTASKS_HTTP_ADDR") {
		t.Errorf("error = %v", err)
	}
}

func TestServeRegistersWithCoreAndAnswersUntilContextEnds(t *testing.T) {
	core := coretest.NewFakeCore(t)
	db := dbtest.NewMigratedSchema(t)
	schema := db.Config().ConnConfig.RuntimeParams["search_path"]
	address := freeLocalAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)

	go func() { stopped <- command.RunServe(ctx, serveEnv(t, core, address), schema) }()
	waitForHealth(t, address)
	deadline := time.Now().Add(5 * time.Second)
	for len(core.Registrations()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()

	if err := <-stopped; err != nil {
		t.Fatalf("serve: %v", err)
	}
	registrations := core.Registrations()
	if len(registrations) != 1 || registrations[0].ID != "gentasks" || len(registrations[0].Permissions) != len(access.Permissions) ||
		registrations[0].Version != command.Version {
		t.Errorf("registrations = %+v", registrations)
	}
}

func TestServeStopsWhenCoreRejectsTheModule(t *testing.T) {
	core := coretest.NewFakeCore(t)
	core.RejectRegistrations(http.StatusConflict, "incompatible_core_version")
	db := dbtest.NewMigratedSchema(t)
	schema := db.Config().ConnConfig.RuntimeParams["search_path"]

	err := command.RunServe(context.Background(), serveEnv(t, core, freeLocalAddress(t)), schema)

	if !errors.Is(err, registration.ErrRegistrationRejected) {
		t.Errorf("error = %v", err)
	}
}
