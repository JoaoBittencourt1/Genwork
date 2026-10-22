package hostmetrics

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// SampleInterval is how often the host is read; snapshots carry it so clients refresh at the same pace.
const SampleInterval = 3 * time.Second

// FirstSampleDelay is the shortest window that gives a meaningful first CPU and network measurement.
const FirstSampleDelay = time.Second

type HostSnapshotSource interface {
	LatestSnapshot() (HostSnapshot, bool)
}

type HostSamplerConfig struct {
	Reader           HostReader
	Interval         time.Duration
	FirstSampleDelay time.Duration
}

// HostSampler reads the host on a fixed interval and keeps the latest snapshot in memory; genmonitor stores no history.
type HostSampler struct {
	config      HostSamplerConfig
	previous    HostReading
	mutex       sync.RWMutex
	latest      HostSnapshot
	hasSnapshot bool
}

func NewHostSampler(config HostSamplerConfig) *HostSampler {
	return &HostSampler{config: config}
}

// Start fails when the host cannot be read, so a missing mount stops the module at startup instead of serving empty metrics.
func (sampler *HostSampler) Start(ctx context.Context) error {
	reading, err := sampler.config.Reader.ReadHost()
	if err != nil {
		return err
	}
	sampler.previous = reading
	if !waitFor(ctx, sampler.config.FirstSampleDelay) {
		return ctx.Err()
	}
	if err := sampler.sample(); err != nil {
		return err
	}
	go sampler.sampleUntilDone(ctx)
	return nil
}

func (sampler *HostSampler) LatestSnapshot() (HostSnapshot, bool) {
	sampler.mutex.RLock()
	defer sampler.mutex.RUnlock()
	return sampler.latest, sampler.hasSnapshot
}

func (sampler *HostSampler) sampleUntilDone(ctx context.Context) {
	ticker := time.NewTicker(sampler.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sampler.sample(); err != nil {
				slog.WarnContext(ctx, "read host metrics", "err", err)
			}
		}
	}
}

func (sampler *HostSampler) sample() error {
	reading, err := sampler.config.Reader.ReadHost()
	if err != nil {
		return err
	}
	snapshot := NewHostSnapshot(sampler.previous, reading)
	sampler.previous = reading
	sampler.mutex.Lock()
	defer sampler.mutex.Unlock()
	sampler.latest, sampler.hasSnapshot = snapshot, true
	return nil
}

func waitFor(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
