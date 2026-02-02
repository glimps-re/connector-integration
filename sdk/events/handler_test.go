package events

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/glimps-re/connector-integration/sdk/metrics"
)

type notifierMock struct {
	notifyMock func(ctx context.Context, event any) (err error)
}

func (m notifierMock) Notify(ctx context.Context, event any) (err error) {
	if m.notifyMock != nil {
		return m.notifyMock(ctx, event)
	}
	panic("mock not implemented")
}

func TestNewHandler(t *testing.T) {
	type args struct {
		notifier        Notifier
		logLeveler      slog.Leveler
		unresolvedError map[ErrorEventType]string
	}
	tests := []struct {
		name  string
		args  args
		wantH *Handler
	}{
		{
			name: "ok",
			args: args{
				notifier:        notifierMock{},
				logLeveler:      &slog.LevelVar{},
				unresolvedError: map[ErrorEventType]string{GMalwareError: "error"},
			},
			wantH: &Handler{
				logHandler: LogHandler{
					eventPusher: notifierMock{},
				},
				notifier: notifierMock{},
				errors:   map[ErrorEventType]string{GMalwareError: "error"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotH := NewHandler(tt.args.notifier, &slog.LevelVar{}, tt.args.unresolvedError, &metrics.MetricsCollector{})
			if !reflect.DeepEqual(gotH.errors, tt.wantH.errors) {
				t.Errorf("NewHandler() = %v, want %v", gotH, tt.wantH)
			}
		})
	}
}
