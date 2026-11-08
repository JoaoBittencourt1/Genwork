package sandbox

import (
	"context"
	"sync"
)

// engineCapacity is what the engine can give one container. The engine refuses a CPU limit above its CPU count, so a
// small machine (a one-CPU VPS, a Docker Desktop VM) gets the sandbox limits trimmed to fit instead of failing.
type engineCapacity struct {
	mutex  sync.Mutex
	known  bool
	cpus   float64
	memory int64
}

func (capacity *engineCapacity) read(ctx context.Context, engine *engineClient) (float64, int64, bool) {
	capacity.mutex.Lock()
	defer capacity.mutex.Unlock()
	if capacity.known {
		return capacity.cpus, capacity.memory, true
	}
	var info struct {
		NCPU     int   `json:"NCPU"`
		MemTotal int64 `json:"MemTotal"`
	}
	if _, err := engine.do(ctx, "GET", "/info", nil, nil, &info); err != nil || info.NCPU < 1 {
		return 0, 0, false
	}
	capacity.known, capacity.cpus, capacity.memory = true, float64(info.NCPU), info.MemTotal
	return capacity.cpus, capacity.memory, true
}

// fitLimits trims the spec's limits to the engine's capacity; when the engine does not say, the limits stay as they are.
func (sandbox *ContainerSandbox) fitLimits(ctx context.Context, spec Spec) Spec {
	cpus, memory, known := sandbox.capacity.read(ctx, sandbox.engine)
	if !known {
		return spec
	}
	if spec.Limits.CPUs > cpus {
		spec.Limits.CPUs = cpus
	}
	if memory > 0 && spec.Limits.Memory > memory {
		spec.Limits.Memory = memory
	}
	return spec
}
