package hostmetrics_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetricstest"
)

var readAt = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return readAt }

func readHost(t *testing.T, procPath, rootPath string) hostmetrics.HostReading {
	t.Helper()
	reading, err := hostmetrics.NewProcHostReader(procPath, rootPath, fixedClock).ReadHost()
	if err != nil {
		t.Fatalf("read host: %v", err)
	}
	return reading
}

func TestProcHostReaderReadsTheRecordedHost(t *testing.T) {
	reading := readHost(t, hostmetricstest.ProcPath(), hostmetricstest.RootPath())

	if !reading.ReadAt.Equal(readAt) || reading.LogicalCPUs != hostmetricstest.RecordedLogicalCPUs || reading.Uptime != hostmetricstest.RecordedUptime {
		t.Errorf("read at %v, %d CPUs, uptime %v", reading.ReadAt, reading.LogicalCPUs, reading.Uptime)
	}
	if reading.CPU != (hostmetrics.CPUTimes{Busy: hostmetricstest.RecordedCPUBusyTicks, Total: hostmetricstest.RecordedCPUTotalTicks}) {
		t.Errorf("cpu = %+v", reading.CPU)
	}
	wantMemory := hostmetrics.MemoryUsage{TotalBytes: hostmetricstest.RecordedMemoryTotalBytes, AvailableBytes: hostmetricstest.RecordedMemoryAvailableBytes}
	if reading.Memory != wantMemory {
		t.Errorf("memory = %+v", reading.Memory)
	}
	wantNetwork := hostmetrics.NetworkCounters{
		Interface: hostmetricstest.RecordedInterface, ReceivedBytes: hostmetricstest.RecordedReceivedBytes, TransmittedBytes: hostmetricstest.RecordedTransmittedBytes,
	}
	if reading.Network != wantNetwork {
		t.Errorf("network = %+v", reading.Network)
	}
	wantOperatingSystem := hostmetrics.OperatingSystem{
		Name: hostmetricstest.RecordedOperatingSystemName, KernelRelease: hostmetricstest.RecordedKernelRelease, Hostname: hostmetricstest.RecordedHostname,
	}
	if reading.OperatingSystem != wantOperatingSystem {
		t.Errorf("operating system = %+v", reading.OperatingSystem)
	}
}

func TestProcHostReaderMeasuresTheRootFileSystem(t *testing.T) {
	disk := readHost(t, hostmetricstest.ProcPath(), hostmetricstest.RootPath()).Disk

	if disk.TotalBytes == 0 || disk.UsedBytes > disk.TotalBytes || disk.AvailableBytes > disk.TotalBytes {
		t.Errorf("disk = %+v", disk)
	}
}

func TestProcHostReaderLeavesOperatingSystemEmptyWithoutItsFiles(t *testing.T) {
	procPath := hostmetricstest.NewProcTree(t, map[string]string{"sys/kernel/osrelease": ""})

	reading := readHost(t, procPath, t.TempDir())

	if reading.OperatingSystem != (hostmetrics.OperatingSystem{}) {
		t.Errorf("operating system = %+v", reading.OperatingSystem)
	}
}

func TestProcHostReaderFallsBackToTheOperatingSystemName(t *testing.T) {
	rootPath := hostmetricstest.NewRootTree(t, map[string]string{"etc/os-release": "NAME='Alpine Linux'\nID=alpine\n"})

	if name := readHost(t, hostmetricstest.ProcPath(), rootPath).OperatingSystem.Name; name != "Alpine Linux" {
		t.Errorf("name = %q", name)
	}
}

func TestProcHostReaderReportsNoInterfaceWithoutADefaultRoute(t *testing.T) {
	procPath := hostmetricstest.NewProcTree(t, map[string]string{
		"1/net/route": "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
			"eth0\t0002A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n",
	})

	if network := readHost(t, procPath, hostmetricstest.RootPath()).Network; network != (hostmetrics.NetworkCounters{}) {
		t.Errorf("network = %+v", network)
	}
}

func TestProcHostReaderFailsWithoutAProcfs(t *testing.T) {
	_, err := hostmetrics.NewProcHostReader(t.TempDir(), hostmetricstest.RootPath(), fixedClock).ReadHost()

	if err == nil || !strings.Contains(err.Error(), "read host stat") {
		t.Errorf("error = %v", err)
	}
}

func TestProcHostReaderFailsWithoutTheRootFileSystem(t *testing.T) {
	_, err := hostmetrics.NewProcHostReader(hostmetricstest.ProcPath(), "/genmonitor/missing/root", fixedClock).ReadHost()

	if err == nil || !strings.Contains(err.Error(), "disk usage") {
		t.Errorf("error = %v", err)
	}
}

func TestProcHostReaderRejectsMalformedFiles(t *testing.T) {
	cases := map[string]map[string]string{
		"stat without aggregate line": {"stat": "cpu0 1 2 3 4 5 6 7 8\n"},
		"short cpu line":              {"stat": "cpu  1 2 3\n"},
		"non-numeric cpu ticks":       {"stat": "cpu  1 2 3 four 5 6 7 8\n"},
		"meminfo without available":   {"meminfo": "MemTotal: 2048 kB\n"},
		"default interface not in dev": {
			"1/net/dev": "Inter-| Receive | Transmit\n face |bytes packets|bytes packets\n    lo: 1 2 3 4 5 6 7 8 9 10\n",
		},
		"short dev line":       {"1/net/dev": "  eth0: 1 2 3\n"},
		"non-numeric dev line": {"1/net/dev": "  eth0: many 2 3 4 5 6 7 8 9 10\n"},
		"empty uptime":         {"uptime": ""},
		"non-numeric uptime":   {"uptime": "forever 1\n"},
	}
	for name, replacements := range cases {
		t.Run(name, func(t *testing.T) {
			procPath := hostmetricstest.NewProcTree(t, replacements)
			if _, err := hostmetrics.NewProcHostReader(procPath, hostmetricstest.RootPath(), fixedClock).ReadHost(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestProcHostReaderNamesTheMissingProcFile(t *testing.T) {
	for _, relativePath := range []string{"meminfo", "1/net/route", "1/net/dev", "uptime"} {
		t.Run(relativePath, func(t *testing.T) {
			procPath := hostmetricstest.NewProcTree(t, nil)
			if err := os.Remove(filepath.Join(procPath, relativePath)); err != nil {
				t.Fatalf("remove %s: %v", relativePath, err)
			}
			_, err := hostmetrics.NewProcHostReader(procPath, hostmetricstest.RootPath(), fixedClock).ReadHost()
			if err == nil || !strings.Contains(err.Error(), relativePath) {
				t.Errorf("error = %v", err)
			}
		})
	}
}
