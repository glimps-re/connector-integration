package events

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

type EventLogHandler interface {
	GetLogHandler() slog.Handler
}

type LogEvent struct {
	Level   string `json:"level" validate:"required,oneof=error warn info debug"`
	Message string `json:"message" validate:"required"`
	Time    int64  `json:"time" validate:"required"`
}

type LogHandler struct {
	eventPusher Notifier
	leveler     slog.Leveler
}

func (h *Handler) GetLogHandler() slog.Handler {
	return h.logHandler
}

func (c LogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if c.leveler != nil {
		minLevel = c.leveler.Level()
	}
	return level >= minLevel
}

func (c LogHandler) Handle(ctx context.Context, record slog.Record) (err error) {
	log := LogEvent{
		Time:    record.Time.Unix(),
		Message: record.Message,
		Level:   strings.ToLower(record.Level.String()),
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*15)
	defer cancel()

	err = c.eventPusher.Notify(ctx, log)
	if err != nil {
		return
	}
	return
}

func (c LogHandler) WithAttrs(attrs []slog.Attr) (newHandler slog.Handler) {
	return c
}

func (c LogHandler) WithGroup(name string) (newHandler slog.Handler) {
	return c
}
