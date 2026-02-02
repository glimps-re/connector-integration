package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/glimps-re/connector-integration/sdk/metrics"
)

func TestLogHandler(t *testing.T) {
	tests := []struct {
		name             string
		log              func(logger *slog.Logger)
		logLevel         slog.Level
		wantEvent        LogEvent
		wantNotifyCalled bool
	}{
		{
			name: "ok",
			log: func(logger *slog.Logger) {
				l1 := logger.With(slog.String("attr1", "toto"), slog.Int("attr2", 1))
				subLogger := l1.WithGroup("group1")
				subLogger.Info("toto", slog.String("test", "nested toto"))
			},
			wantEvent: LogEvent{
				Level:   "info",
				Message: "toto",
				Attributes: map[string]any{
					"attr1": "toto",
					"attr2": int64(1),
					"group1": map[string]any{
						"test": "nested toto",
					},
				},
			},
			wantNotifyCalled: true,
		},
		{
			name:     "ok debug",
			logLevel: slog.LevelDebug,
			log: func(logger *slog.Logger) {
				l1 := logger.With(slog.String("attr1", "toto"), slog.Int("attr2", 1))
				subLogger := l1.WithGroup("group1")
				l2 := subLogger.With(slog.Any("attr3", []string{"test1", "test2"}))
				l2.WithGroup("group2").WithGroup("group3").Debug("toto debug", slog.String("attr4", "test4"), "test", "toto") //nolint:sloglint // made on purpose to check that it's ok
			},
			wantEvent: LogEvent{
				Level:   "debug",
				Message: "toto debug",
				Attributes: map[string]any{
					"attr1": "toto",
					"attr2": int64(1),
					"group1": map[string]any{
						"attr3": []string{"test1", "test2"},
						"group2": map[string]any{
							"group3": map[string]any{
								"attr4": "test4",
								"test":  "toto",
							},
						},
					},
				},
			},
			wantNotifyCalled: true,
		},
		{
			name:     "ok duplicate arg key",
			logLevel: slog.LevelDebug,
			log: func(logger *slog.Logger) {
				subLogger := logger.With(slog.String("arg1", "test1"))
				subLogger.Debug("toto debug", slog.String("arg1", "test2"))
			},
			wantEvent: LogEvent{
				Level:   "debug",
				Message: "toto debug",
				Attributes: map[string]any{
					"arg1": "test2",
				},
			},
			wantNotifyCalled: true,
		},
		{
			name:     "ok duplicate arg/group key",
			logLevel: slog.LevelDebug,
			log: func(logger *slog.Logger) {
				subLogger := logger.With(slog.String("group1", "test1"))
				subLogger.WithGroup("group1").Debug("toto debug", slog.String("arg1", "test1"))
			},
			wantEvent: LogEvent{
				Level:   "debug",
				Message: "toto debug",
				Attributes: map[string]any{
					"group1": map[string]any{
						"arg1": "test1",
					},
				},
			},
			wantNotifyCalled: true,
		},
		{
			name:     "ok sub loggers from same parent", // to check their own attributes are preserved
			logLevel: slog.LevelDebug,
			log: func(logger *slog.Logger) {
				originalHandler := logger.Handler().(*LogHandler)
				attrs := make([]attrWithGroups, 1, 2) // force spare capacity to guarantee append() will write into same backing array
				attrs[0] = attrWithGroups{attr: slog.String("common", "val")}
				handler := LogHandler{
					eventPusher: originalHandler.eventPusher,
					leveler:     originalHandler.leveler,
					attributes:  attrs,
				}
				parent := slog.New(handler)
				sub1 := parent.With(slog.String("sub", "1"))
				_ = parent.With(slog.String("sub", "2")) // to check this operation does not overwrite sub1's attribute in the shared backing array
				sub1.Debug("msg")
			},
			wantEvent: LogEvent{
				Level:   "debug",
				Message: "msg",
				Attributes: map[string]any{
					"common": "val",
					"sub":    "1",
				},
			},
			wantNotifyCalled: true,
		},
		{
			name:     "ok sub loggers from same parent with groups", // to check their own groups are preserved
			logLevel: slog.LevelDebug,
			log: func(logger *slog.Logger) {
				originalHandler := logger.Handler().(*LogHandler)
				groups := make([]string, 1, 2) // force spare capacity to guarantee append() will write into same backing array
				groups[0] = "g1"
				handler := LogHandler{
					eventPusher: originalHandler.eventPusher,
					leveler:     originalHandler.leveler,
					groups:      groups,
				}
				parent := slog.New(handler)
				sub1 := parent.WithGroup("sub1")
				_ = parent.WithGroup("sub2") // to check this operation does not overwrite sub1's group in the shared backing array
				sub1.Debug("msg", slog.String("key", "val"))
			},
			wantEvent: LogEvent{
				Level:   "debug",
				Message: "msg",
				Attributes: map[string]any{
					"g1": map[string]any{
						"sub1": map[string]any{
							"key": "val",
						},
					},
				},
			},
			wantNotifyCalled: true,
		},
		{
			name: "ok no attributes",
			log: func(logger *slog.Logger) {
				logger.Info("hello")
			},
			wantEvent: LogEvent{
				Level:      "info",
				Message:    "hello",
				Attributes: map[string]any{},
			},
			wantNotifyCalled: true,
		},
		{
			name:     "ok debug not called",
			logLevel: slog.LevelInfo,
			log: func(logger *slog.Logger) {
				logger.Debug("toto debug", slog.String("arg1", "test1"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifyCalled := false
			notifier := notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					ev, ok := event.(LogEvent)
					if !ok {
						t.Errorf("event is not log event")
					}
					if diff := cmp.Diff(ev, tt.wantEvent, cmpopts.IgnoreFields(LogEvent{}, "Time")); diff != "" {
						t.Fatalf("Notifier diff in received log, diff(got-want)=%s", diff)
					}
					notifyCalled = true
					return
				},
			}
			leveler := &slog.LevelVar{}
			leveler.Set(tt.logLevel)

			h := NewHandler(notifier, leveler, map[ErrorEventType]string{}, &metrics.MetricsCollector{})
			l := slog.New(h.GetLogHandler())
			tt.log(l)

			if notifyCalled != tt.wantNotifyCalled {
				t.Errorf("notify called: %v, want %v", notifyCalled, tt.wantNotifyCalled)
			}
		})
	}
}
