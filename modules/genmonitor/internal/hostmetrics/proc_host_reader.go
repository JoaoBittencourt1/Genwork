package hostmetrics

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ProcHostReader reads the host through its procfs and root file system, which the container mounts read-only.
type ProcHostReader struct {
	procPath string
	rootPath string
	now      func() time.Time
}

func NewProcHostReader(procPath, rootPath string, now func() time.Time) *ProcHostReader {
	return &ProcHostReader{procPath: procPath, rootPath: rootPath, now: now}
}

func (reader *ProcHostReader) ReadHost() (HostReading, error) {
	reading := HostReading{ReadAt: reader.now(), OperatingSystem: reader.readOperatingSystem()}
	var err error
	if reading.CPU, reading.LogicalCPUs, err = reader.readCPU(); err != nil {
		return HostReading{}, err
	}
	if reading.Memory, err = reader.readMemory(); err != nil {
		return HostReading{}, err
	}
	if reading.Disk, err = reader.readDisk(); err != nil {
		return HostReading{}, err
	}
	if reading.Network, err = reader.readNetwork(); err != nil {
		return HostReading{}, err
	}
	if reading.Uptime, err = reader.readUptime(); err != nil {
		return HostReading{}, err
	}
	return reading, nil
}

func (reader *ProcHostReader) readProcFile(relativePath string) (string, error) {
	content, err := os.ReadFile(filepath.Join(reader.procPath, relativePath))
	if err != nil {
		return "", fmt.Errorf("read host %s: %w", relativePath, err)
	}
	return string(content), nil
}

func (reader *ProcHostReader) readCPU() (CPUTimes, int, error) {
	stat, err := reader.readProcFile("stat")
	if err != nil {
		return CPUTimes{}, 0, err
	}
	return parseCPUTimes(stat)
}

func (reader *ProcHostReader) readMemory() (MemoryUsage, error) {
	meminfo, err := reader.readProcFile("meminfo")
	if err != nil {
		return MemoryUsage{}, err
	}
	return parseMemoryUsage(meminfo)
}

func (reader *ProcHostReader) readDisk() (DiskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(reader.rootPath, &stat); err != nil {
		return DiskUsage{}, fmt.Errorf("read host disk usage: %w", err)
	}
	blockSize := uint64(stat.Bsize)
	return DiskUsage{
		TotalBytes:     stat.Blocks * blockSize,
		UsedBytes:      counterDelta(stat.Bfree, stat.Blocks) * blockSize,
		AvailableBytes: stat.Bavail * blockSize,
	}, nil
}

// readNetwork goes through PID 1, which lives in the host's network namespace; procfs's own net/ follows the reader's, the container's.
func (reader *ProcHostReader) readNetwork() (NetworkCounters, error) {
	routes, err := reader.readProcFile("1/net/route")
	if err != nil {
		return NetworkCounters{}, err
	}
	defaultInterface := parseDefaultRouteInterface(routes)
	if defaultInterface == "" {
		return NetworkCounters{}, nil
	}
	devices, err := reader.readProcFile("1/net/dev")
	if err != nil {
		return NetworkCounters{}, err
	}
	return parseNetworkCounters(devices, defaultInterface)
}

func (reader *ProcHostReader) readUptime() (time.Duration, error) {
	uptime, err := reader.readProcFile("uptime")
	if err != nil {
		return 0, err
	}
	return parseUptime(uptime)
}

// readOperatingSystem leaves a field empty when its file is missing: distributions differ, and the rest is still worth showing.
func (reader *ProcHostReader) readOperatingSystem() OperatingSystem {
	kernelRelease, _ := reader.readProcFile("sys/kernel/osrelease")
	osRelease, _ := os.ReadFile(filepath.Join(reader.rootPath, "etc", "os-release"))
	hostname, _ := os.ReadFile(filepath.Join(reader.rootPath, "etc", "hostname"))
	return OperatingSystem{
		Name:          parseOperatingSystemName(string(osRelease)),
		KernelRelease: strings.TrimSpace(kernelRelease),
		Hostname:      strings.TrimSpace(string(hostname)),
	}
}

// parseCPUTimes reads the aggregate "cpu" line of /proc/stat; guest time is left out because user time already includes it.
func parseCPUTimes(stat string) (CPUTimes, int, error) {
	var times CPUTimes
	found := false
	logicalCPUs := 0
	scanner := bufio.NewScanner(strings.NewReader(stat))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		switch {
		case fields[0] == "cpu":
			parsed, err := parseAggregateCPULine(fields[1:])
			if err != nil {
				return CPUTimes{}, 0, err
			}
			times, found = parsed, true
		case strings.HasPrefix(fields[0], "cpu"):
			logicalCPUs++
		}
	}
	if !found {
		return CPUTimes{}, 0, errors.New("parse host stat: no aggregate cpu line")
	}
	return times, logicalCPUs, nil
}

func parseAggregateCPULine(fields []string) (CPUTimes, error) {
	const userNiceSystemIdleIowaitIrqSoftirqSteal = 8
	const idleIndex, iowaitIndex = 3, 4
	if len(fields) < userNiceSystemIdleIowaitIrqSoftirqSteal {
		return CPUTimes{}, errors.New("parse host stat: cpu line is too short")
	}
	var total, idle uint64
	for index, field := range fields[:userNiceSystemIdleIowaitIrqSoftirqSteal] {
		ticks, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return CPUTimes{}, fmt.Errorf("parse host stat: %w", err)
		}
		total += ticks
		if index == idleIndex || index == iowaitIndex {
			idle += ticks
		}
	}
	return CPUTimes{Busy: total - idle, Total: total}, nil
}

func parseMemoryUsage(meminfo string) (MemoryUsage, error) {
	kibibytes := map[string]uint64{}
	scanner := bufio.NewScanner(strings.NewReader(meminfo))
	for scanner.Scan() {
		name, rest, found := strings.Cut(scanner.Text(), ":")
		fields := strings.Fields(rest)
		if !found || len(fields) == 0 {
			continue
		}
		if value, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
			kibibytes[name] = value
		}
	}
	total, hasTotal := kibibytes["MemTotal"]
	available, hasAvailable := kibibytes["MemAvailable"]
	if !hasTotal || !hasAvailable {
		return MemoryUsage{}, errors.New("parse host meminfo: MemTotal or MemAvailable is missing")
	}
	return MemoryUsage{TotalBytes: total * 1024, AvailableBytes: available * 1024}, nil
}

// parseDefaultRouteInterface picks the interface of the lowest-metric default route, the one carrying the host's traffic.
func parseDefaultRouteInterface(routes string) string {
	const ifaceIndex, destinationIndex, metricIndex, maskIndex = 0, 1, 6, 7
	chosen, chosenMetric := "", uint64(0)
	scanner := bufio.NewScanner(strings.NewReader(routes))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= maskIndex || fields[destinationIndex] != "00000000" || fields[maskIndex] != "00000000" {
			continue
		}
		metric, err := strconv.ParseUint(fields[metricIndex], 10, 64)
		if err == nil && (chosen == "" || metric < chosenMetric) {
			chosen, chosenMetric = fields[ifaceIndex], metric
		}
	}
	return chosen
}

func parseNetworkCounters(devices, networkInterface string) (NetworkCounters, error) {
	const receivedBytesIndex, transmittedBytesIndex = 0, 8
	scanner := bufio.NewScanner(strings.NewReader(devices))
	for scanner.Scan() {
		name, counters, found := strings.Cut(scanner.Text(), ":")
		if !found || strings.TrimSpace(name) != networkInterface {
			continue
		}
		fields := strings.Fields(counters)
		if len(fields) <= transmittedBytesIndex {
			return NetworkCounters{}, fmt.Errorf("parse host net/dev: %s line is too short", networkInterface)
		}
		received, receivedErr := strconv.ParseUint(fields[receivedBytesIndex], 10, 64)
		transmitted, transmittedErr := strconv.ParseUint(fields[transmittedBytesIndex], 10, 64)
		if err := errors.Join(receivedErr, transmittedErr); err != nil {
			return NetworkCounters{}, fmt.Errorf("parse host net/dev: %w", err)
		}
		return NetworkCounters{Interface: networkInterface, ReceivedBytes: received, TransmittedBytes: transmitted}, nil
	}
	return NetworkCounters{}, fmt.Errorf("parse host net/dev: interface %s not found", networkInterface)
}

func parseUptime(uptime string) (time.Duration, error) {
	fields := strings.Fields(uptime)
	if len(fields) == 0 {
		return 0, errors.New("parse host uptime: file is empty")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse host uptime: %w", err)
	}
	return time.Duration(seconds * float64(time.Second)).Truncate(time.Second), nil
}

func parseOperatingSystemName(osRelease string) string {
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(osRelease))
	for scanner.Scan() {
		if key, value, found := strings.Cut(scanner.Text(), "="); found {
			values[key] = strings.Trim(value, `"'`)
		}
	}
	if values["PRETTY_NAME"] != "" {
		return values["PRETTY_NAME"]
	}
	return values["NAME"]
}
