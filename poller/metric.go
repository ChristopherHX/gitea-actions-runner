package poller

import "sync/atomic"

// Metric interface
type Metric interface {
	IncBusyWorker() uint64
	DecBusyWorker() uint64
	BusyWorkers() uint64
}

var _ Metric = (*metric)(nil)

type metric struct {
	busyWorkers uint64
}

// NewMetric for default metric structure
func NewMetric() Metric {
	return &metric{}
}

func (m *metric) IncBusyWorker() uint64 {
	return atomic.AddUint64(&m.busyWorkers, 1)
}

func (m *metric) DecBusyWorker() uint64 {
	return atomic.AddUint64(&m.busyWorkers, ^uint64(0))
}

func (m *metric) BusyWorkers() uint64 {
	return atomic.LoadUint64(&m.busyWorkers)
}
