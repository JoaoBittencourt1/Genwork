// Package hostmetrics samples the machine genmonitor runs on: CPU, memory, disk, network, and operating system.
package hostmetrics

import "time"

// CPUTimes are clock ticks summed over every CPU since boot; usage only exists as the difference between two readings.
type CPUTimes struct {
	Busy  uint64
	Total uint64
}

type MemoryUsage struct {
	TotalBytes     uint64
	AvailableBytes uint64
}

func (memory MemoryUsage) UsedBytes() uint64 {
	return counterDelta(memory.AvailableBytes, memory.TotalBytes)
}

type DiskUsage struct {
	TotalBytes     uint64
	UsedBytes      uint64
	AvailableBytes uint64
}

// NetworkCounters are the bytes an interface moved since it came up; an empty Interface means the host has no default route.
type NetworkCounters struct {
	Interface        string
	ReceivedBytes    uint64
	TransmittedBytes uint64
}

type OperatingSystem struct {
	Name          string
	KernelRelease string
	Hostname      string
}

type HostReading struct {
	ReadAt          time.Time
	LogicalCPUs     int
	CPU             CPUTimes
	Memory          MemoryUsage
	Disk            DiskUsage
	Network         NetworkCounters
	OperatingSystem OperatingSystem
	Uptime          time.Duration
}

type HostReader interface {
	ReadHost() (HostReading, error)
}

// counterDelta is zero when a counter went backwards, as it does when an interface is recreated.
func counterDelta(previous, current uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}
