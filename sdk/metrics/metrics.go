package metrics

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/glimps-re/go-gdetect/pkg/gdetect"
)

// Allow connectors to record metrics (items_processed, size_processed and error_items; other metrics are automatically collected).
// The methods are thread-safe.
type MetricCollecter interface {
	// add +1 to total items processed (an item can be a file, an email or other) and +size to total size processed
	AddItemProcessed(size int64)
	// add +1 to total items in error (whatever the reason, for example failed to analyze)
	AddErrorItem()
}

var _ MetricCollecter = &MetricsCollector{}

// MetricsCollector provides thread-safe methods for collecting connector metrics.
type MetricsCollector struct {
	// counters
	itemsProcessed atomic.Int64
	sizeProcessed  atomic.Int64
	mitigatedItems atomic.Int64 // automatically collected
	errorItems     atomic.Int64
	// gauges
	dailyQuota          atomic.Int64 // automatically collected
	availableDailyQuota atomic.Int64 // automatically collected
	runningSince        atomic.Int64 // automatically collected

	detectClient gdetect.GDetectSubmitter
}

// ConnectorMetrics represents current state of connector metrics.
type ConnectorMetrics struct {
	// no omitempty tags, so metrics are always explicit
	DailyQuota          int64 `json:"daily_quota"`
	AvailableDailyQuota int64 `json:"available_daily_quota"`
	RunningSince        int64 `json:"running_since" desc:"Unix time (in seconds) when connector last registered"`
	ItemsProcessed      int64 `json:"items_processed" desc:"number of items processed"`
	SizeProcessed       int64 `json:"size_processed" desc:"total size processed, in bytes"`
	MitigatedItems      int64 `json:"mitigated_items"`
	ErrorItems          int64 `json:"error_items" desc:"items in error, for X reason"`
}

func (m *MetricsCollector) AddItemProcessed(size int64) {
	m.itemsProcessed.Add(1)
	m.sizeProcessed.Add(size)
}

func (m *MetricsCollector) AddErrorItem() {
	m.errorItems.Add(1)
}

func (m *MetricsCollector) AddMitigatedItem() {
	m.mitigatedItems.Add(1)
}

// SetDetectClient sets the gdetect client used to retrieve quotas.
func (m *MetricsCollector) SetDetectClient(client gdetect.GDetectSubmitter) {
	m.detectClient = client
}

// GetAndStoreQuotas retrieves quotas from gdetect API and stores them.
func (m *MetricsCollector) GetAndStoreQuotas(ctx context.Context) (err error) {
	getStatusCtx, getStatusCancel := context.WithTimeout(ctx, 15*time.Second) // on purpose large timeout
	defer getStatusCancel()
	status, err := m.detectClient.GetProfileStatus(getStatusCtx)
	if err != nil {
		err = fmt.Errorf("could not get status to retrieve quotas, %w", err)
		return
	}
	m.dailyQuota.Store(int64(status.DailyQuota))
	m.availableDailyQuota.Store(int64(status.AvailableDailyQuota))
	return
}

func (m *MetricsCollector) SetRunningSince(rs int64) {
	m.runningSince.Store(rs)
}

// GetAndReset returns current metrics and resets counters to zero.
// Gauges are read without reset.
func (m *MetricsCollector) GetAndReset() (metrics ConnectorMetrics) {
	metrics = ConnectorMetrics{
		// counters
		ItemsProcessed: m.itemsProcessed.Swap(0),
		SizeProcessed:  m.sizeProcessed.Swap(0),
		MitigatedItems: m.mitigatedItems.Swap(0),
		ErrorItems:     m.errorItems.Swap(0),
		// gauges
		DailyQuota:          m.dailyQuota.Load(),
		AvailableDailyQuota: m.availableDailyQuota.Load(),
		RunningSince:        m.runningSince.Load(),
	}
	return
}

// RestoreCounterMetrics adds given metrics values to current counters.
// Gauges are not restored since they are not reset on read.
func (m *MetricsCollector) RestoreCounterMetrics(metrics ConnectorMetrics) {
	m.itemsProcessed.Add(metrics.ItemsProcessed)
	m.sizeProcessed.Add(metrics.SizeProcessed)
	m.mitigatedItems.Add(metrics.MitigatedItems)
	m.errorItems.Add(metrics.ErrorItems)
}
