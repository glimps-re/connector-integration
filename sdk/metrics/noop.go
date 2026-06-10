package metrics

var _ MetricCollecter = NoopMetricCollecter{}

// NoopMetricCollecter discards all metrics. To use as default when no connector-manager is wired.
type NoopMetricCollecter struct{}

func (NoopMetricCollecter) AddItemProcessed(size int64) {}

func (NoopMetricCollecter) AddErrorItem() {}
