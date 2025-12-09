package events

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

			h := NewHandler(notifier, leveler, map[ErrorEventType]string{})
			l := slog.New(h.GetLogHandler())
			tt.log(l)

			if notifyCalled != tt.wantNotifyCalled {
				t.Errorf("notify called: %v, want %v", notifyCalled, tt.wantNotifyCalled)
			}
		})
	}
}
