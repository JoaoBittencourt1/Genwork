package hostmetrics_test

import (
	"math"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetricstest"
)

func approximately(value, want float64) bool {
	return math.Abs(value-want) < 1e-9
}

func TestNewHostSnapshotMeasuresCPUAndNetworkSinceThePreviousReading(t *testing.T) {
	current := hostmetricstest.BusyReading()

	snapshot := hostmetrics.NewHostSnapshot(hostmetricstest.BaselineReading(), current)

	if !approximately(snapshot.CPU.UsedPercent, 30) || snapshot.CPU.LogicalCPUs != current.LogicalCPUs {
		t.Errorf("cpu = %+v", snapshot.CPU)
	}
	network := snapshot.Network
	if !approximately(network.ReceivedBytesPerSecond, 1000) || !approximately(network.TransmittedBytesPerSecond, 250) {
		t.Errorf("throughput = %+v", network)
	}
	if network.Interface != "eth0" || network.ReceivedBytes != current.Network.ReceivedBytes || network.TransmittedBytes != current.Network.TransmittedBytes {
		t.Errorf("network = %+v", network)
	}
	if !snapshot.SampledAt.Equal(current.ReadAt) || snapshot.Uptime != current.Uptime || snapshot.OperatingSystem != current.OperatingSystem {
		t.Errorf("snapshot = %+v", snapshot)
	}
	if snapshot.Memory != current.Memory || snapshot.Disk != current.Disk {
		t.Errorf("memory = %+v, disk = %+v", snapshot.Memory, snapshot.Disk)
	}
}

func TestNewHostSnapshotHasNoThroughputWhenTheInterfaceChanged(t *testing.T) {
	current := hostmetricstest.BusyReading()
	current.Network.Interface = "wlan0"

	network := hostmetrics.NewHostSnapshot(hostmetricstest.BaselineReading(), current).Network

	if network.ReceivedBytesPerSecond != 0 || network.TransmittedBytesPerSecond != 0 || network.Interface != "wlan0" {
		t.Errorf("network = %+v", network)
	}
}

func TestNewHostSnapshotHasNoThroughputWithoutElapsedTime(t *testing.T) {
	current := hostmetricstest.BusyReading()
	current.ReadAt = hostmetricstest.BaselineReadAt

	network := hostmetrics.NewHostSnapshot(hostmetricstest.BaselineReading(), current).Network

	if network.ReceivedBytesPerSecond != 0 || network.TransmittedBytesPerSecond != 0 {
		t.Errorf("network = %+v", network)
	}
}

func TestNewHostSnapshotTreatsCountersThatWentBackwardsAsIdle(t *testing.T) {
	current := hostmetricstest.BusyReading()
	current.CPU = hostmetrics.CPUTimes{Busy: 10, Total: 100}
	current.Network.ReceivedBytes, current.Network.TransmittedBytes = 10, 10

	snapshot := hostmetrics.NewHostSnapshot(hostmetricstest.BaselineReading(), current)

	if snapshot.CPU.UsedPercent != 0 || snapshot.Network.ReceivedBytesPerSecond != 0 || snapshot.Network.TransmittedBytesPerSecond != 0 {
		t.Errorf("snapshot = %+v", snapshot)
	}
}

func TestNewHostSnapshotCapsCPUUsageAtOneHundredPercent(t *testing.T) {
	previous := hostmetricstest.BaselineReading()
	current := hostmetricstest.BusyReading()
	current.CPU = hostmetrics.CPUTimes{Busy: previous.CPU.Busy + 500, Total: previous.CPU.Total + 100}

	if used := hostmetrics.NewHostSnapshot(previous, current).CPU.UsedPercent; used != 100 {
		t.Errorf("used = %v", used)
	}
}

func TestMemoryUsedBytesIsTotalMinusAvailable(t *testing.T) {
	if used := (hostmetrics.MemoryUsage{TotalBytes: 8 << 30, AvailableBytes: 6 << 30}).UsedBytes(); used != 2<<30 {
		t.Errorf("used = %d", used)
	}
	if used := (hostmetrics.MemoryUsage{TotalBytes: 1, AvailableBytes: 2}).UsedBytes(); used != 0 {
		t.Errorf("used = %d", used)
	}
}

func TestNewHostSnapshotResponseUsesWholeSecondsAndUTC(t *testing.T) {
	snapshot := hostmetrics.NewHostSnapshot(hostmetricstest.BaselineReading(), hostmetricstest.BusyReading())
	snapshot.SampledAt = snapshot.SampledAt.In(time.FixedZone("BRT", -3*60*60))

	response := hostmetrics.NewHostSnapshotResponse(snapshot, 3*time.Second)

	if response.SampleIntervalSeconds != 3 || response.UptimeSeconds != int64((26*time.Hour+2*time.Second)/time.Second) {
		t.Errorf("response = %+v", response)
	}
	if response.SampledAt.Location() != time.UTC || response.Memory.UsedBytes != 2<<30 || response.Disk.UsedBytes != 40<<30 {
		t.Errorf("response = %+v", response)
	}
}
