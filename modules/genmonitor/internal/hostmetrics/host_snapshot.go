package hostmetrics

import "time"

type CPUUsage struct {
	LogicalCPUs int
	UsedPercent float64
}

type NetworkThroughput struct {
	Interface                 string
	ReceivedBytesPerSecond    float64
	TransmittedBytesPerSecond float64
	ReceivedBytes             uint64
	TransmittedBytes          uint64
}

type HostSnapshot struct {
	SampledAt       time.Time
	OperatingSystem OperatingSystem
	Uptime          time.Duration
	CPU             CPUUsage
	Memory          MemoryUsage
	Disk            DiskUsage
	Network         NetworkThroughput
}

// NewHostSnapshot describes the host at the current reading, with CPU usage and throughput measured since the previous one.
func NewHostSnapshot(previous, current HostReading) HostSnapshot {
	return HostSnapshot{
		SampledAt:       current.ReadAt,
		OperatingSystem: current.OperatingSystem,
		Uptime:          current.Uptime,
		CPU:             CPUUsage{LogicalCPUs: current.LogicalCPUs, UsedPercent: cpuUsedPercent(previous.CPU, current.CPU)},
		Memory:          current.Memory,
		Disk:            current.Disk,
		Network:         networkThroughput(previous, current),
	}
}

func cpuUsedPercent(previous, current CPUTimes) float64 {
	total := counterDelta(previous.Total, current.Total)
	if total == 0 {
		return 0
	}
	busy := min(counterDelta(previous.Busy, current.Busy), total)
	return float64(busy) * 100 / float64(total)
}

func networkThroughput(previous, current HostReading) NetworkThroughput {
	throughput := NetworkThroughput{
		Interface:        current.Network.Interface,
		ReceivedBytes:    current.Network.ReceivedBytes,
		TransmittedBytes: current.Network.TransmittedBytes,
	}
	elapsedSeconds := current.ReadAt.Sub(previous.ReadAt).Seconds()
	if elapsedSeconds <= 0 || previous.Network.Interface != current.Network.Interface {
		return throughput
	}
	throughput.ReceivedBytesPerSecond = float64(counterDelta(previous.Network.ReceivedBytes, current.Network.ReceivedBytes)) / elapsedSeconds
	throughput.TransmittedBytesPerSecond = float64(counterDelta(previous.Network.TransmittedBytes, current.Network.TransmittedBytes)) / elapsedSeconds
	return throughput
}
