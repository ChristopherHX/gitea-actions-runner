package poller

import "sync/atomic"

// Metric interface
type Metric interface {
	IncBusyWorker() int64
	DecBusyWorker() int64
	BusyWorkers() int64
}

var _ Metric = (*metric)(nil)

type metric struct {
	busyWorkers int64
}

// NewMetric for default metric structure
func NewMetric() Metric {
	return &metric{}
}

func (m *metric) IncBusyWorker() int64 {
	return atomic.AddInt64(&m.busyWorkers, 1)
}

func (m *metric) DecBusyWorker() int64 {
	return atomic.AddInt64(&m.busyWorkers, -1)
}

func (m *metric) BusyWorkers() int64 {
	return atomic.LoadInt64(&m.busyWorkers)
}
