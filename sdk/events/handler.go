package events

import (
	"context"
	"log/slog"
	"sync"
)

type EventHandler interface {
	EventLogHandler
	EventErrorHandler
	EventMitigationHandler
}

var _ EventHandler = &Handler{}

type Event interface {
	MitigationEvent | TaskEvent | LogEvent | ErrorEvent | ResolutionEvent
}

type EventType string

const (
	TaskAck    EventType = "task"
	Mitigation EventType = "mitigation"
	Log        EventType = "log"
	Error      EventType = "error"
	Resolution EventType = "resolution"
)

type Notifier interface {
	// event MUST be an `Event``
	Notify(ctx context.Context, event any) (err error)
}

type Handler struct {
	logHandler slog.Handler
	notifier   Notifier
	errors     map[ErrorEventType]string
	lock       sync.Mutex
}

func NewHandler(notifier Notifier, logLeveler slog.Leveler, unresolvedError map[ErrorEventType]string) (h *Handler) {
	if unresolvedError == nil {
		unresolvedError = make(map[ErrorEventType]string)
	}
	return &Handler{
		logHandler: &LogHandler{
			eventPusher: notifier,
			leveler:     logLeveler,
		},
		notifier: notifier,
		errors:   unresolvedError,
	}
}
