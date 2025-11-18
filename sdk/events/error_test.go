package events

import (
	"context"
	"errors"
	"testing"
)

func TestConsoleEventHandler_PushError(t *testing.T) {
	tests := []struct {
		name        string
		errorType   ErrorEventType
		e           error
		eventPusher Notifier
		errors      map[ErrorEventType]string
		wantErr     bool
	}{
		{
			name:      "first error seen + push error",
			errorType: GMalwareError,
			e:         errors.New("test error"),
			eventPusher: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("wanted push error")
				},
			},
			errors:  map[ErrorEventType]string{},
			wantErr: true,
		},
		{
			name:      "already seen error",
			errorType: GMalwareError,
			e:         errors.New("test error"),
			eventPusher: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("push event must nos been called")
				},
			},
			errors: map[ErrorEventType]string{GMalwareError: "test error"},
		},
		{
			name:      "first error seen + ok",
			errorType: GMalwareError,
			e:         errors.New("test error"),
			eventPusher: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return
				},
			},
			errors: map[ErrorEventType]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Handler{
				notifier: tt.eventPusher,
				errors:   tt.errors,
			}
			gotErr := h.NotifyError(context.Background(), tt.errorType, tt.e)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("PushError() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("PushError() succeeded unexpectedly")
			}
		})
	}
}

func TestConsoleEventHandler_PushResolution(t *testing.T) {
	tests := []struct {
		name        string
		errorType   ErrorEventType
		msg         string
		eventPusher Notifier
		errors      map[ErrorEventType]string
		wantErr     bool
	}{
		{
			name:      "first error seen + push error",
			errorType: GMalwareError,
			msg:       "test resolv",
			eventPusher: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("wanted push error")
				},
			},
			errors:  map[ErrorEventType]string{GMalwareError: "error"},
			wantErr: true,
		},
		{
			name:      "already seen error",
			errorType: GMalwareError,
			msg:       "test resolv",
			eventPusher: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return errors.New("push event must nos been called")
				},
			},
			errors: map[ErrorEventType]string{},
		},
		{
			name:      "first error seen + ok",
			errorType: GMalwareError,
			msg:       "test resolv",
			eventPusher: notifierMock{
				notifyMock: func(ctx context.Context, event any) (err error) {
					return
				},
			},
			errors: map[ErrorEventType]string{GMalwareError: "error"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Handler{
				notifier: tt.eventPusher,
				errors:   tt.errors,
			}
			gotErr := h.NotifyResolution(context.Background(), tt.msg, tt.errorType)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("PushResolution() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("PushResolution() succeeded unexpectedly")
			}
		})
	}
}
