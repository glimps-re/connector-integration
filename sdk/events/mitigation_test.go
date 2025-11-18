package events

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

func TestHandler_NotifyFileMitigation(t *testing.T) {
	type args struct {
		action    MitigationAction
		elementID string
		reason    MitigationReason
		info      FileInfos
	}
	tests := []struct {
		name     string
		notifier Notifier
		args     args
		wantErr  bool
	}{
		{
			name: "error",
			notifier: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("test want error")
				},
			},
			args: args{
				action:    ActionQuarantine,
				elementID: "id",
				reason:    ReasonMalware,
				info:      FileInfos{},
			},
			wantErr: true,
		},
		{
			name: "ok",
			notifier: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					ev, ok := event.(MitigationEvent)
					if !ok {
						t.Fatal("invalid event. want MitigationEvent")
					}
					_, ok = ev.Info.(FileInfos)
					if !ok {
						t.Fatal("invalid event info. want FileDetails")
					}
					return
				},
			},
			args: args{
				action:    ActionBlock,
				elementID: "id",
				reason:    ReasonMalware,
				info:      FileInfos{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.notifier, &slog.LevelVar{}, map[ErrorEventType]string{})
			if err := h.NotifyFileMitigation(t.Context(), tt.args.action, tt.args.elementID, tt.args.reason, tt.args.info); (err != nil) != tt.wantErr {
				t.Errorf("Handler.notifyMitigation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_NotifyEmailMitigation(t *testing.T) {
	type args struct {
		action    MitigationAction
		elementID string
		reason    MitigationReason
		info      EmailInfos
	}
	tests := []struct {
		name     string
		notifier Notifier
		args     args
		wantErr  bool
	}{
		{
			name: "error",
			notifier: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("test want error")
				},
			},
			args: args{
				action:    ActionQuarantine,
				elementID: "id",
				reason:    ReasonMalware,
				info:      EmailInfos{},
			},
			wantErr: true,
		},
		{
			name: "ok",
			notifier: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					ev, ok := event.(MitigationEvent)
					if !ok {
						t.Fatal("invalid event. want MitigationEvent")
					}
					_, ok = ev.Info.(EmailInfos)
					if !ok {
						t.Fatal("invalid event info. want EmailDetails")
					}
					return
				},
			},
			args: args{
				action:    ActionBlock,
				elementID: "id",
				reason:    ReasonMalware,
				info:      EmailInfos{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.notifier, &slog.LevelVar{}, map[ErrorEventType]string{})
			if err := h.NotifyEmailMitigation(t.Context(), tt.args.action, tt.args.elementID, tt.args.reason, tt.args.info); (err != nil) != tt.wantErr {
				t.Errorf("Handler.notifyMitigation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_NotifyURLMitigation(t *testing.T) {
	type args struct {
		action    MitigationAction
		elementID string
		reason    MitigationReason
		info      URLInfos
	}
	tests := []struct {
		name     string
		notifier Notifier
		args     args
		wantErr  bool
	}{
		{
			name: "error",
			notifier: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("test want error")
				},
			},
			args: args{
				action:    ActionQuarantine,
				elementID: "id",
				reason:    ReasonMalware,
				info:      URLInfos{},
			},
			wantErr: true,
		},
		{
			name: "ok",
			notifier: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					ev, ok := event.(MitigationEvent)
					if !ok {
						t.Fatal("invalid event. want MitigationEvent")
					}
					_, ok = ev.Info.(URLInfos)
					if !ok {
						t.Fatal("invalid event info. want URLDetails")
					}
					return
				},
			},
			args: args{
				action:    ActionBlock,
				elementID: "id",
				reason:    ReasonMalware,
				info:      URLInfos{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.notifier, &slog.LevelVar{}, map[ErrorEventType]string{})
			if err := h.NotifyURLMitigation(t.Context(), tt.args.action, tt.args.elementID, tt.args.reason, tt.args.info); (err != nil) != tt.wantErr {
				t.Errorf("Handler.notifyMitigation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
