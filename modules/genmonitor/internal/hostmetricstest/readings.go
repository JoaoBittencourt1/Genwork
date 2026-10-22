package hostmetricstest

import (
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
)

var BaselineReadAt = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func BaselineReading() hostmetrics.HostReading {
	return hostmetrics.HostReading{
		ReadAt:      BaselineReadAt,
		LogicalCPUs: 4,
		CPU:         hostmetrics.CPUTimes{Busy: 2000, Total: 10000},
		Memory:      hostmetrics.MemoryUsage{TotalBytes: 8 << 30, AvailableBytes: 6 << 30},
		Disk:        hostmetrics.DiskUsage{TotalBytes: 100 << 30, UsedBytes: 40 << 30, AvailableBytes: 55 << 30},
		Network:     hostmetrics.NetworkCounters{Interface: "eth0", ReceivedBytes: 1_000_000, TransmittedBytes: 500_000},
		OperatingSystem: hostmetrics.OperatingSystem{
			Name: RecordedOperatingSystemName, KernelRelease: RecordedKernelRelease, Hostname: RecordedHostname,
		},
		Uptime: 26 * time.Hour,
	}
}

// BusyReading is BaselineReading two seconds later: 30% CPU, 1000 bytes/s received, and 250 bytes/s sent in between.
func BusyReading() hostmetrics.HostReading {
	reading := BaselineReading()
	reading.ReadAt = BaselineReadAt.Add(2 * time.Second)
	reading.CPU = hostmetrics.CPUTimes{Busy: 2300, Total: 11000}
	reading.Network.ReceivedBytes += 2000
	reading.Network.TransmittedBytes += 500
	reading.Uptime += 2 * time.Second
	return reading
}
