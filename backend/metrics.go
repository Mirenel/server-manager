package main

import (
	"sync"
	"time"
)

type MetricPoint struct {
	TimestampMS int64   `json:"timestamp_ms"`
	CPU         float64 `json:"cpu"`
	MemMB       float64 `json:"mem_mb"`
}

type MetricsRingBuffer struct {
	points [3600]MetricPoint
	head   int
	count  int
	mu     sync.Mutex
}

// Push adds a new metric point to the ring buffer
func (mrb *MetricsRingBuffer) Push(cpu, memMB float64) {
	mrb.mu.Lock()
	defer mrb.mu.Unlock()

	if mrb.count < len(mrb.points) {
		mrb.count++
	} else {
		mrb.head = (mrb.head + 1) % len(mrb.points)
	}

	idx := (mrb.head + mrb.count - 1) % len(mrb.points)
	mrb.points[idx] = MetricPoint{
		TimestampMS: time.Now().UnixMilli(),
		CPU:         cpu,
		MemMB:       memMB,
	}
}

// Last returns the last n metric points in chronological order
func (mrb *MetricsRingBuffer) Last(n int) []MetricPoint {
	mrb.mu.Lock()
	defer mrb.mu.Unlock()

	if n > mrb.count {
		n = mrb.count
	}

	result := make([]MetricPoint, n)
	for i := 0; i < n; i++ {
		idx := (mrb.head + mrb.count - n + i) % len(mrb.points)
		result[i] = mrb.points[idx]
	}
	return result
}
