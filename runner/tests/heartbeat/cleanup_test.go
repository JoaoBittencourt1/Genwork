package heartbeat_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/runner/internal/api"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/genagentstest"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/heartbeat"
	"github.com/JoaoBittencourt1/Genwork/runner/internal/sandboxtest"
)

func TestARequestedCleanupRunsOnceAndIsReportedOnTheNextHeartbeat(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	var runs atomic.Int32
	loop := heartbeat.NewLoop(api.NewClient(fake.URL(), genagentstest.Secret, nil), sandboxtest.NewFakeLauncher(nil), heartbeat.Settings{
		Image: "registry.example/genwork-sandbox:2.3.0", MaxConcurrency: 2, ProfilesDir: t.TempDir(),
		Cleanup: func(context.Context) (int64, error) {
			runs.Add(1)
			return 2 << 30, nil
		},
	})
	fake.RequestCleanup()
	ctx := context.Background()

	if err := loop.Beat(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	for range 3 {
		if err := loop.Beat(ctx); err != nil {
			t.Fatal(err)
		}
	}

	heartbeats := fake.HeartbeatsSnapshot()
	if heartbeats[0]["cleanup"] != nil {
		t.Errorf("the first heartbeat reported a cleanup: %v", heartbeats[0])
	}
	report, _ := heartbeats[1]["cleanup"].(map[string]any)
	if report["freedBytes"] != float64(2<<30) {
		t.Errorf("second heartbeat = %v", heartbeats[1])
	}
	if heartbeats[2]["cleanup"] != nil || heartbeats[3]["cleanup"] != nil || runs.Load() != 1 {
		t.Errorf("the cleanup ran %d times; later heartbeats = %v, %v", runs.Load(), heartbeats[2], heartbeats[3])
	}
}

func TestARunnerThatCannotCleanUpIgnoresTheRequest(t *testing.T) {
	fake := genagentstest.NewFakeGenagents(t)
	loop := heartbeat.NewLoop(api.NewClient(fake.URL(), genagentstest.Secret, nil), sandboxtest.NewFakeLauncher(nil), heartbeat.Settings{
		Image: "registry.example/genwork-sandbox:2.3.0", MaxConcurrency: 2, ProfilesDir: t.TempDir(),
	})
	fake.RequestCleanup()

	for range 2 {
		if err := loop.Beat(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	if fake.HeartbeatsSnapshot()[1]["cleanup"] != nil {
		t.Error("a runner without a cleanup reported one")
	}
}
