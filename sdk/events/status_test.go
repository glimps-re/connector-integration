package events

import (
	"context"
	"errors"
	"testing"
)

func TestHandler_NotifyStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     ConnectorLifecycleStatus
		notifyMock func(ctx context.Context, event any) (err error)
		wantErr    bool
	}{
		{
			name:   "started",
			status: StatusStarted,
			notifyMock: func(_ context.Context, event any) error {
				ev, ok := event.(StatusEvent)
				if !ok {
					t.Fatalf("expected StatusEvent, got %T", event)
				}
				if ev.Status != StatusStarted {
					t.Fatalf("expected %q, got %q", StatusStarted, ev.Status)
				}
				return nil
			},
		},
		{
			name:   "stopping",
			status: StatusStopping,
			notifyMock: func(_ context.Context, event any) error {
				if event.(StatusEvent).Status != StatusStopping {
					t.Fatalf("expected %q", StatusStopping)
				}
				return nil
			},
		},
		{
			name:   "stopped",
			status: StatusStopped,
			notifyMock: func(_ context.Context, event any) error {
				if event.(StatusEvent).Status != StatusStopped {
					t.Fatalf("expected %q", StatusStopped)
				}
				return nil
			},
		},
		{
			name:   "notify error is propagated",
			status: StatusStopped,
			notifyMock: func(_ context.Context, _ any) error {
				return errors.New("push failed")
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(notifierMock{notifyMock: tt.notifyMock}, nil, nil, nil)
			err := h.NotifyStatus(context.Background(), tt.status)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NotifyStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
