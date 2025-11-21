package events

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"
)

type EventLogHandler interface {
	GetLogHandler() slog.Handler
}

type LogEvent struct {
	Level      string         `json:"level" validate:"required,oneof=error warn info debug"`
	Message    string         `json:"message" validate:"required"`
	Time       int64          `json:"time" validate:"required"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type attrWithGroups struct {
	attr   slog.Attr
	groups []string
}

type LogHandler struct {
	eventPusher Notifier
	leveler     slog.Leveler
	attributes  []attrWithGroups
	groups      []string
}

func (h *Handler) GetLogHandler() slog.Handler {
	return h.logHandler
}

func (lh LogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if lh.leveler != nil {
		minLevel = lh.leveler.Level()
	}
	return level >= minLevel
}

func (lh LogHandler) Handle(ctx context.Context, record slog.Record) (err error) {
	log := LogEvent{
		Time:       record.Time.Unix(),
		Message:    record.Message,
		Level:      strings.ToLower(record.Level.String()),
		Attributes: lh.getAttributes(record),
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*15)
	defer cancel()

	err = lh.eventPusher.Notify(ctx, log)
	if err != nil {
		return
	}
	return
}

func (lh LogHandler) WithAttrs(attrs []slog.Attr) (newHandler slog.Handler) {
	if len(attrs) == 0 {
		return lh
	}
	lh.attributes = slices.Clip(lh.attributes) // to force append() to allocate a new backing array, to prevent sub handlers from overwriting each other's data
	groups := slices.Clip(lh.groups)
	for _, attr := range attrs {
		lh.attributes = append(lh.attributes, attrWithGroups{
			attr:   attr,
			groups: groups,
		})
	}
	return lh
}

func (lh LogHandler) WithGroup(name string) (newHandler slog.Handler) {
	if name == "" {
		return lh
	}
	lh.groups = slices.Clip(lh.groups)
	lh.groups = append(lh.groups, name)
	return lh
}

func (lh LogHandler) getAttributes(record slog.Record) (attributes map[string]any) {
	attributes = make(map[string]any)
	for _, a := range lh.attributes {
		addAttribute(attributes, a.groups, a.attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		addAttribute(attributes, lh.groups, attr)
		return true
	})
	return
}

func addAttribute(attributes map[string]any, groups []string, attr slog.Attr) {
	if len(groups) == 0 {
		attributes[attr.Key] = attr.Value.Any()
		return
	}

	current := attributes
	for _, group := range groups {
		if rawGroupMap, exists := current[group]; exists {
			if groupMap, ok := rawGroupMap.(map[string]any); ok {
				current = groupMap
				continue
			}
		}
		groupMap := make(map[string]any)
		current[group] = groupMap
		current = groupMap
	}
	current[attr.Key] = attr.Value.Any()
}
