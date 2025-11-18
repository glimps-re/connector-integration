package events

import (
	"context"
	"time"
)

type EventErrorHandler interface {
	NotifyError(ctx context.Context, errorType ErrorEventType, e error) (err error)
	NotifyResolution(ctx context.Context, message string, errorTypes ...ErrorEventType) (err error)
}

type ErrorEvent struct {
	Error string         `json:"error" validate:"required"`
	Type  ErrorEventType `json:"type" validate:"required"`
	Time  int64          `json:"time" validate:"required"`
}

type ResolutionEvent struct {
	Types      []ErrorEventType `json:"type" validate:"required"`
	Resolution string           `json:"resolution" validate:"required"`
	Time       int64            `json:"time" validate:"required"`
}

type ErrorEventType string

const (
	// MUST be used for all gmalware detect client error
	GMalwareError ErrorEventType = "gmalware"
	// MUST be used for gmalware configuration or reconfiguration error
	GMalwareConfigError ErrorEventType = "gmalware-bad-config"
)

// e MUST NOT be nil
func (h *Handler) NotifyError(ctx context.Context, errorType ErrorEventType, e error) (err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.hadError(errorType, e.Error()) {
		return
	}
	errEvent := ErrorEvent{
		Error: e.Error(),
		Type:  errorType,
		Time:  time.Now().Unix(),
	}
	err = h.notifier.Notify(ctx, errEvent)
	if err != nil {
		return
	}
	h.errors[errorType] = e.Error()
	return
}

func (h *Handler) NotifyResolution(ctx context.Context, msg string, errorTypes ...ErrorEventType) (err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if !h.hadErrors(errorTypes) {
		return
	}
	resEvent := ResolutionEvent{
		Resolution: msg,
		Types:      errorTypes,
		Time:       time.Now().Unix(),
	}
	err = h.notifier.Notify(ctx, resEvent)
	if err != nil {
		return
	}
	for _, errType := range errorTypes {
		delete(h.errors, errType)
	}
	return
}

// MUST be used under RLock
func (h *Handler) hadErrors(errorTypes []ErrorEventType) bool {
	for _, errType := range errorTypes {
		if _, ok := h.errors[errType]; ok {
			return true
		}
	}
	return false
}

// MUST be used under RLock
func (h *Handler) hadError(errorType ErrorEventType, errorMsg string) bool {
	if msg, ok := h.errors[errorType]; ok && msg == errorMsg {
		return true
	}
	return false
}
