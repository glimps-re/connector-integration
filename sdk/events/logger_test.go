package events

import (
	"context"
	"log/slog"
	"testing"
)

func TestLogHandler(t *testing.T) {
	type args struct {
		notifier Notifier
		msg      string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "ok",
			args: args{
				notifier: notifierMock{
					notifyMock: func(ctx context.Context, event any) (err error) {
						_, ok := event.(LogEvent)
						if !ok {
							t.Errorf("event is not log event")
						}
						return
					},
				},
				msg: "message",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.args.notifier, &slog.LevelVar{}, map[ErrorEventType]string{})
			l := slog.New(h.GetLogHandler())
			l.Info(tt.args.msg)
		})
	}
}
