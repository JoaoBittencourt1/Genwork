package hostmetrics

import "time"

type OperatingSystemResponse struct {
	Name          string `json:"name"`
	KernelRelease string `json:"kernelRelease"`
	Hostname      string `json:"hostname"`
}

type CPUUsageResponse struct {
	LogicalCPUs int     `json:"logicalCpus"`
	UsedPercent float64 `json:"usedPercent"`
}

type StorageUsageResponse struct {
	TotalBytes     uint64 `json:"totalBytes"`
	UsedBytes      uint64 `json:"usedBytes"`
	AvailableBytes uint64 `json:"availableBytes"`
}

type NetworkThroughputResponse struct {
	Interface                 string  `json:"interface"`
	ReceivedBytesPerSecond    float64 `json:"receivedBytesPerSecond"`
	TransmittedBytesPerSecond float64 `json:"transmittedBytesPerSecond"`
	ReceivedBytes             uint64  `json:"receivedBytes"`
	TransmittedBytes          uint64  `json:"transmittedBytes"`
}

type HostSnapshotResponse struct {
	SampledAt             time.Time                 `json:"sampledAt"`
	SampleIntervalSeconds int                       `json:"sampleIntervalSeconds"`
	UptimeSeconds         int64                     `json:"uptimeSeconds"`
	OperatingSystem       OperatingSystemResponse   `json:"operatingSystem"`
	CPU                   CPUUsageResponse          `json:"cpu"`
	Memory                StorageUsageResponse      `json:"memory"`
	Disk                  StorageUsageResponse      `json:"disk"`
	Network               NetworkThroughputResponse `json:"network"`
}

func NewHostSnapshotResponse(snapshot HostSnapshot, sampleInterval time.Duration) HostSnapshotResponse {
	return HostSnapshotResponse{
		SampledAt:             snapshot.SampledAt.UTC(),
		SampleIntervalSeconds: int(sampleInterval / time.Second),
		UptimeSeconds:         int64(snapshot.Uptime / time.Second),
		OperatingSystem: OperatingSystemResponse{
			Name:          snapshot.OperatingSystem.Name,
			KernelRelease: snapshot.OperatingSystem.KernelRelease,
			Hostname:      snapshot.OperatingSystem.Hostname,
		},
		CPU: CPUUsageResponse{LogicalCPUs: snapshot.CPU.LogicalCPUs, UsedPercent: snapshot.CPU.UsedPercent},
		Memory: StorageUsageResponse{
			TotalBytes:     snapshot.Memory.TotalBytes,
			UsedBytes:      snapshot.Memory.UsedBytes(),
			AvailableBytes: snapshot.Memory.AvailableBytes,
		},
		Disk: StorageUsageResponse{
			TotalBytes:     snapshot.Disk.TotalBytes,
			UsedBytes:      snapshot.Disk.UsedBytes,
			AvailableBytes: snapshot.Disk.AvailableBytes,
		},
		Network: NetworkThroughputResponse{
			Interface:                 snapshot.Network.Interface,
			ReceivedBytesPerSecond:    snapshot.Network.ReceivedBytesPerSecond,
			TransmittedBytesPerSecond: snapshot.Network.TransmittedBytesPerSecond,
			ReceivedBytes:             snapshot.Network.ReceivedBytes,
			TransmittedBytes:          snapshot.Network.TransmittedBytes,
		},
	}
}
