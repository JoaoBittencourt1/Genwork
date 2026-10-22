package hostmetricstest

import (
	"sync"

	"github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/hostmetrics"
)

// FakeHostReader returns its scripted readings in order, then repeats the last one; after Fail it returns the error instead.
type FakeHostReader struct {
	mutex    sync.Mutex
	readings []hostmetrics.HostReading
	failure  error
	reads    int
}

func NewFakeHostReader(readings ...hostmetrics.HostReading) *FakeHostReader {
	return &FakeHostReader{readings: readings}
}

func (reader *FakeHostReader) ReadHost() (hostmetrics.HostReading, error) {
	reader.mutex.Lock()
	defer reader.mutex.Unlock()
	reader.reads++
	if reader.failure != nil {
		return hostmetrics.HostReading{}, reader.failure
	}
	index := min(reader.reads, len(reader.readings)) - 1
	return reader.readings[index], nil
}

func (reader *FakeHostReader) Fail(err error) {
	reader.mutex.Lock()
	defer reader.mutex.Unlock()
	reader.failure = err
}

func (reader *FakeHostReader) Reads() int {
	reader.mutex.Lock()
	defer reader.mutex.Unlock()
	return reader.reads
}
