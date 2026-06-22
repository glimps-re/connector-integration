package events

import "context"

// EventStatusHandler reports connector lifecycle transitions to the manager.
type EventStatusHandler interface {
	NotifyStatus(ctx context.Context, status ConnectorLifecycleStatus) (err error)
}

// ConnectorLifecycleStatus is the lifecycle state a connector reports.
type ConnectorLifecycleStatus string

const (
	// StatusStarted indicates the connector is running and accepting files.
	StatusStarted ConnectorLifecycleStatus = "started"
	// StatusStopping indicates the connector is draining in-flight analyses before it stops.
	StatusStopping ConnectorLifecycleStatus = "stopping"
	// StatusStopped indicates the connector is fully stopped and idle.
	StatusStopped ConnectorLifecycleStatus = "stopped"
)

// StatusEvent reports a connector lifecycle transition to the connector manager.
type StatusEvent struct {
	Status ConnectorLifecycleStatus `json:"status" validate:"required,oneof=started stopping stopped"`
}

// NotifyStatus sends the connector lifecycle status to the connector manager.
func (h *Handler) NotifyStatus(ctx context.Context, status ConnectorLifecycleStatus) (err error) {
	return h.notifier.Notify(ctx, StatusEvent{Status: status})
}
