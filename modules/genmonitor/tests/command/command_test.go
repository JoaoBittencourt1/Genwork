package command_test

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/access"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/command"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetricstest"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/sdk/go/coretest"
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

func serveEnv(core *coretest.FakeCore, address, procPath string) platform.Getenv {
	return envFrom(map[string]string{
		"GENMONITOR_HTTP_ADDR":  address,
		"CORE_INTERNAL_URL":     core.URL(),
		"GENWORK_MODULE_SECRET": coretest.ModuleSecret,
		"GENWORK_PUBLIC_URL":    coretest.Issuer,
		"GENMONITOR_HOST_PROC":  procPath,
		"GENMONITOR_HOST_ROOT":  hostmetricstest.RootPath(),
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
	t.Fatal("genmonitor did not become healthy")
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

func TestMigrateSucceedsWithoutADatabase(t *testing.T) {
	if err := command.Run(context.Background(), []string{"migrate"}, envFrom(nil)); err != nil {
		t.Errorf("migrate: %v", err)
	}
}

func TestServeFailsFastOnInvalidConfiguration(t *testing.T) {
	if err := command.Run(context.Background(), []string{"serve"}, envFrom(nil)); err == nil || !strings.Contains(err.Error(), "GENMONITOR_HOST_PROC") {
		t.Errorf("error = %v", err)
	}
}

func TestServeFailsWhenTheHostCannotBeRead(t *testing.T) {
	core := coretest.NewFakeCore(t)

	err := command.RunServe(context.Background(), serveEnv(core, freeLocalAddress(t), t.TempDir()))

	if err == nil || !strings.Contains(err.Error(), "sample host") {
		t.Errorf("error = %v", err)
	}
}

func TestServeRegistersWithCoreAndAnswersUntilContextEnds(t *testing.T) {
	core := coretest.NewFakeCore(t)
	address := freeLocalAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)

	go func() {
		stopped <- command.Run(ctx, []string{"serve"}, serveEnv(core, address, hostmetricstest.ProcPath()))
	}()
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
	if len(registrations) != 1 || registrations[0].ID != "genmonitor" || len(registrations[0].Permissions) != len(access.Permissions) ||
		registrations[0].Version != command.Version {
		t.Errorf("registrations = %+v", registrations)
	}
}

func TestServeStopsWhenCoreRejectsTheRegistration(t *testing.T) {
	core := coretest.NewFakeCore(t)
	core.RejectRegistrations(http.StatusConflict, "incompatible_core_version")

	err := command.RunServe(context.Background(), serveEnv(core, freeLocalAddress(t), hostmetricstest.ProcPath()))

	if err == nil || !strings.Contains(err.Error(), "register with core") {
		t.Errorf("error = %v", err)
	}
}
