package events

import (
	"context"
	"log/slog"
)

var _ EventHandler = NoopEventHandler{}

type NoopEventHandler struct{}

func (h NoopEventHandler) NotifyError(ctx context.Context, errorType ErrorEventType, e error) (err error) {
	return
}

func (h NoopEventHandler) NotifyResolution(ctx context.Context, msg string, errorTypes ...ErrorEventType) (err error) {
	return
}

func (h NoopEventHandler) NotifyFileMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info FileInfos) (err error) {
	return
}

func (h NoopEventHandler) NotifyEmailMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info EmailInfos) (err error) {
	return
}

func (h NoopEventHandler) NotifyURLMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info URLInfos) (err error) {
	return
}

func (h NoopEventHandler) GetLogHandler() slog.Handler {
	return slog.DiscardHandler
}
