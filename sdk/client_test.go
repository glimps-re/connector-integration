package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glimps-re/connector-integration/sdk/events"
)

func TestConnectorManagerClient_Notify_StatusEvent(t *testing.T) {
	var got postEventRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unmarshal request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewConnectorManagerClient(context.Background(), ConnectorManagerClientConfig{URL: srv.URL})
	if err := client.Notify(context.Background(), events.StatusEvent{Status: events.StatusStopping}); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if got.EventType != events.Status {
		t.Fatalf("EventType = %q, want %q", got.EventType, events.Status)
	}
	var ev events.StatusEvent
	if err := json.Unmarshal(got.Event, &ev); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if ev.Status != events.StatusStopping {
		t.Fatalf("Status = %q, want %q", ev.Status, events.StatusStopping)
	}
}
