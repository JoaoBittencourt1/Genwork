package hostmetrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetricstest"
)

func newSampler(reader hostmetrics.HostReader, interval time.Duration) *hostmetrics.HostSampler {
	return hostmetrics.NewHostSampler(hostmetrics.HostSamplerConfig{Reader: reader, Interval: interval, FirstSampleDelay: time.Millisecond})
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHostSamplerHasNoSnapshotBeforeStart(t *testing.T) {
	sampler := newSampler(hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading()), time.Hour)

	if _, found := sampler.LatestSnapshot(); found {
		t.Error("a sampler that never started must have no snapshot")
	}
}

func TestHostSamplerStartPublishesTheFirstSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sampler := newSampler(hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading(), hostmetricstest.BusyReading()), time.Hour)

	if err := sampler.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	snapshot, found := sampler.LatestSnapshot()
	if !found || !approximately(snapshot.CPU.UsedPercent, 30) || !snapshot.SampledAt.Equal(hostmetricstest.BusyReading().ReadAt) {
		t.Errorf("snapshot = %+v, found = %v", snapshot, found)
	}
}

func TestHostSamplerStartFailsWhenTheHostCannotBeRead(t *testing.T) {
	reader := hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading())
	reader.Fail(errors.New("read host stat: no such file"))

	if err := newSampler(reader, time.Hour).Start(context.Background()); err == nil {
		t.Error("expected an error")
	}
}

func TestHostSamplerStartFailsWhenTheSecondReadingFails(t *testing.T) {
	reader := hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading())
	sampler := hostmetrics.NewHostSampler(hostmetrics.HostSamplerConfig{Reader: reader, Interval: time.Hour, FirstSampleDelay: 50 * time.Millisecond})
	started := make(chan error, 1)

	go func() { started <- sampler.Start(context.Background()) }()
	waitUntil(t, func() bool { return reader.Reads() == 1 })
	reader.Fail(errors.New("read host meminfo: permission denied"))

	if err := <-started; err == nil {
		t.Error("expected an error")
	}
}

func TestHostSamplerStartStopsWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sampler := hostmetrics.NewHostSampler(hostmetrics.HostSamplerConfig{
		Reader: hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading()), Interval: time.Hour, FirstSampleDelay: time.Hour,
	})

	if err := sampler.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v", err)
	}
}

func TestHostSamplerKeepsSamplingOnItsInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading(), hostmetricstest.BusyReading())
	sampler := newSampler(reader, time.Millisecond)

	if err := sampler.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	waitUntil(t, func() bool { return reader.Reads() >= 4 })

	if snapshot, _ := sampler.LatestSnapshot(); snapshot.CPU.UsedPercent != 0 {
		t.Errorf("repeating the same reading must measure an idle CPU, got %+v", snapshot.CPU)
	}
}

func TestHostSamplerKeepsTheLastSnapshotWhenAReadingFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := hostmetricstest.NewFakeHostReader(hostmetricstest.BaselineReading(), hostmetricstest.BusyReading())
	sampler := newSampler(reader, time.Millisecond)
	if err := sampler.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	reader.Fail(errors.New("read host stat: input/output error"))
	failedFrom := reader.Reads()
	waitUntil(t, func() bool { return reader.Reads() >= failedFrom+3 })

	if _, found := sampler.LatestSnapshot(); !found {
		t.Error("a failed reading must keep the last snapshot")
	}
}
