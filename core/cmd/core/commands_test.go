package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/core/internal/auth"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform"
	"github.com/JoaoBittencourt1/Genwork/core/internal/platform/dbtest"
)

func envFrom(values map[string]string) platform.Getenv {
	return func(name string) string { return values[name] }
}

func TestRunRejectsUnknownOrIncompleteCommands(t *testing.T) {
	ctx := context.Background()
	for _, args := range [][]string{{}, {"explode"}, {"admin"}, {"admin", "delete"}} {
		err := run(ctx, args, envFrom(nil), strings.NewReader(""), &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "usage: core <command>") {
			t.Errorf("run(%v) error = %v, want usage", args, err)
		}
	}
}

func TestKeygenPrintsAValidSigningKey(t *testing.T) {
	var output bytes.Buffer
	if err := run(context.Background(), []string{"keygen"}, envFrom(nil), nil, &output); err != nil {
		t.Fatal(err)
	}
	seed := strings.TrimSpace(output.String())
	if decoded, err := base64.StdEncoding.DecodeString(seed); err != nil || len(decoded) != 32 {
		t.Fatalf("keygen output %q is not a 32-byte base64 seed", seed)
	}
	if _, err := auth.ParseSigningKey(seed); err != nil {
		t.Fatalf("core cannot load its own generated key: %v", err)
	}
}

func TestWorkerStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx, []string{"worker"}, envFrom(nil), nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("worker error = %v", err)
	}
}

func TestSubcommandsRequireDatabaseURL(t *testing.T) {
	ctx := context.Background()
	if err := runMigrate(ctx, envFrom(nil), "unused"); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("migrate error = %v", err)
	}
	args := []string{"--email", "ana@example.com", "--name", "Ana", "--password-stdin"}
	if err := runAdminCreate(ctx, envFrom(nil), "unused", args, strings.NewReader("correct horse battery\n"), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("admin create error = %v", err)
	}
}

func TestMigrateCreatesSchemaAndIsIdempotent(t *testing.T) {
	databaseURL := dbtest.DatabaseURL(t)
	schema := dbtest.NewSchemaName(t)
	env := envFrom(map[string]string{"DATABASE_URL": databaseURL})
	ctx := context.Background()

	for attempt := 1; attempt <= 2; attempt++ {
		if err := runMigrate(ctx, env, schema); err != nil {
			t.Fatalf("migrate attempt %d: %v", attempt, err)
		}
	}
	pool, err := platform.OpenDatabase(ctx, databaseURL, schema)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var roles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM roles`).Scan(&roles); err != nil || roles != 2 {
		t.Fatalf("roles = %d, err = %v", roles, err)
	}
}

func TestAdminCreateCommandCreatesAdministrator(t *testing.T) {
	pool := dbtest.NewMigratedSchema(t)
	schema := pool.Config().ConnConfig.RuntimeParams["search_path"]
	env := envFrom(map[string]string{"DATABASE_URL": dbtest.DatabaseURL(t)})
	args := []string{"--email", "Ana@Example.com", "--name", "Ana", "--password-stdin"}
	var output bytes.Buffer

	if err := runAdminCreate(context.Background(), env, schema, args, strings.NewReader("correct horse battery\n"), &output); err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if !strings.Contains(output.String(), "administrator created: ana@example.com") {
		t.Errorf("output = %q", output.String())
	}
	err := runAdminCreate(context.Background(), env, schema, args, strings.NewReader("correct horse battery\n"), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "email_already_registered") {
		t.Errorf("duplicate error = %v", err)
	}
}

func TestServeAnswersUntilContextEnds(t *testing.T) {
	pool := dbtest.NewMigratedSchema(t)
	schema := pool.Config().ConnConfig.RuntimeParams["search_path"]
	address := freeLocalAddress(t)
	env := envFrom(map[string]string{
		"DATABASE_URL":           dbtest.DatabaseURL(t),
		"CORE_HTTP_ADDR":         address,
		"GENWORK_PUBLIC_URL":     "http://localhost:8485",
		"GENWORK_INSTANCE_NAME":  "Serve Test",
		"GENWORK_DEFAULT_LOCALE": "en",
		"JWT_SIGNING_KEY":        auth.GenerateSigningKeySeed(),
		"SMTP_HOST":              "127.0.0.1",
		"SMTP_PORT":              "1",
		"SMTP_FROM":              "Genwork <no-reply@genwork.test>",
		"SMTP_TLS":               "none",
	})
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- runServe(ctx, env, schema) }()

	waitForHealth(t, "http://"+address+"/healthz")
	response, err := http.Get("http://" + address + "/.well-known/genwork")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("discovery status = %d", response.StatusCode)
	}

	cancel()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop after cancellation")
	}
}

func TestServeFailsFastOnInvalidConfiguration(t *testing.T) {
	ctx := context.Background()
	if err := runServe(ctx, envFrom(nil), "unused"); err == nil || !strings.Contains(err.Error(), "GENWORK_INSTANCE_NAME") {
		t.Errorf("missing config error = %v", err)
	}
	env := map[string]string{
		"DATABASE_URL": "postgres://unused", "CORE_HTTP_ADDR": ":0", "GENWORK_PUBLIC_URL": "http://localhost",
		"GENWORK_INSTANCE_NAME": "x", "GENWORK_DEFAULT_LOCALE": "en", "JWT_SIGNING_KEY": "not-a-key",
		"SMTP_HOST": "localhost", "SMTP_PORT": "25", "SMTP_FROM": "a@b.c", "SMTP_TLS": "none",
	}
	if err := runServe(ctx, envFrom(env), "unused"); err == nil || !strings.Contains(err.Error(), "JWT_SIGNING_KEY") {
		t.Errorf("invalid key error = %v", err)
	}
}

func freeLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func waitForHealth(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never became healthy", url)
}
