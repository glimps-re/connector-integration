package metrics

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/glimps-re/go-gdetect/pkg/gdetect"
)

// Allow connectors to record metrics (items_processed_total, processed_bytes_total and items_error_total; other metrics are automatically collected).
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
	itemsMitigated atomic.Int64 // automatically collected
	itemsError     atomic.Int64
	// gauges
	dailyQuota          atomic.Int64 // automatically collected
	availableDailyQuota atomic.Int64 // automatically collected
	lastStart           atomic.Int64 // automatically collected

	detectClient gdetect.GDetectSubmitter
}

// ConnectorMetrics represents current state of connector metrics.
type ConnectorMetrics struct {
	// no omitempty tags, so metrics are always explicit
	DailyQuota          int64 `json:"daily_quota"`
	AvailableDailyQuota int64 `json:"available_daily_quota"`
	LastStart           int64 `json:"last_start_timestamp_seconds" desc:"Unix time (in seconds) when connector last registered"`
	ItemsProcessed      int64 `json:"items_processed_total" desc:"number of items processed"`
	SizeProcessed       int64 `json:"processed_bytes_total" desc:"total size processed, in bytes"`
	ItemsMitigated      int64 `json:"items_mitigated_total"`
	ItemsError          int64 `json:"items_error_total" desc:"items in error, for X reason"`
}

func (m *MetricsCollector) AddItemProcessed(size int64) {
	m.itemsProcessed.Add(1)
	m.sizeProcessed.Add(size)
}

func (m *MetricsCollector) AddErrorItem() {
	m.itemsError.Add(1)
}

func (m *MetricsCollector) AddMitigatedItem() {
	m.itemsMitigated.Add(1)
}

// SetDetectClient sets the gdetect client used to retrieve quotas.
func (m *MetricsCollector) SetDetectClient(client gdetect.GDetectSubmitter) {
	m.detectClient = client
}

// GetAndStoreQuotas retrieves quotas from gdetect API and stores them.
func (m *MetricsCollector) GetAndStoreQuotas(ctx context.Context) (err error) {
	if m.detectClient == nil {
		err = errors.New("detect client is nil")
		return
	}
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

func (m *MetricsCollector) SetLastStart(rs int64) {
	m.lastStart.Store(rs)
}

// GetAndReset returns current metrics and resets counters to zero.
// Gauges are read without reset.
func (m *MetricsCollector) GetAndReset() (metrics ConnectorMetrics) {
	metrics = ConnectorMetrics{
		// counters
		ItemsProcessed: m.itemsProcessed.Swap(0),
		SizeProcessed:  m.sizeProcessed.Swap(0),
		ItemsMitigated: m.itemsMitigated.Swap(0),
		ItemsError:     m.itemsError.Swap(0),
		// gauges
		DailyQuota:          m.dailyQuota.Load(),
		AvailableDailyQuota: m.availableDailyQuota.Load(),
		LastStart:           m.lastStart.Load(),
	}
	return
}

// RestoreCounterMetrics adds given metrics values to current counters.
// Gauges are not restored since they are not reset on read.
func (m *MetricsCollector) RestoreCounterMetrics(metrics ConnectorMetrics) {
	m.itemsProcessed.Add(metrics.ItemsProcessed)
	m.sizeProcessed.Add(metrics.SizeProcessed)
	m.itemsMitigated.Add(metrics.ItemsMitigated)
	m.itemsError.Add(metrics.ItemsError)
}
