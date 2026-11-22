package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glimps-re/connector-integration/sdk/events"
	"github.com/glimps-re/connector-integration/sdk/metrics"
)

// fakeConsole serves one queued task per poll, then holds the poll open, and
// records the task acknowledgements.
type fakeConsole struct {
	mu      sync.Mutex
	queue   []Task
	polls   int
	acks    map[string]string
	hangAck chan struct{}
}

func (f *fakeConsole) handler(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == basePath+"/tasks":
		// The server only notices a closed connection once the body is read.
		_, _ = io.Copy(io.Discard, r.Body)
		f.mu.Lock()
		f.polls++
		var out []Task
		if len(f.queue) > 0 {
			out = f.queue[:1]
			f.queue = f.queue[1:]
		}
		f.mu.Unlock()
		if out == nil {
			<-r.Context().Done()
			return
		}
		_ = json.NewEncoder(w).Encode(getTasksResp{Tasks: out})
	case r.Method == http.MethodPost && r.URL.Path == basePath+"/events":
		req := new(postEventRequest)
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(req); err != nil || req.EventType != events.TaskAck {
			w.WriteHeader(http.StatusOK)
			return
		}
		if f.hangAck != nil {
			select {
			case <-f.hangAck:
			case <-r.Context().Done():
			}
			return
		}
		ev := new(events.TaskEvent)
		if err := json.Unmarshal(req.Event, ev); err == nil {
			f.mu.Lock()
			f.acks[ev.TaskID] = ev.Error
			f.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (f *fakeConsole) ackOf(id string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.acks[id]
	return e, ok
}

func (f *fakeConsole) pollCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.polls
}

// blockingConnector blocks in Restore until released.
type blockingConnector struct {
	entered chan struct{}
	release chan struct{}
}

func (b *blockingConnector) Start(context.Context) error                      { return nil }
func (b *blockingConnector) Stop(context.Context) error                       { return nil }
func (b *blockingConnector) Configure(context.Context, json.RawMessage) error { return nil }
func (b *blockingConnector) Status() ConnectorStatus                          { return Started }
func (b *blockingConnector) Restore(context.Context, RestoreActionContent) error {
	close(b.entered)
	<-b.release
	return nil
}

func newTestClient(srv *httptest.Server) ConnectorManagerClient {
	return ConnectorManagerClient{
		httpClient:       srv.Client(),
		url:              srv.URL,
		apiKey:           "key",
		metricsCollector: &metrics.MetricsCollector{},
	}
}

// buffered returns a task that runs, then n tasks that wait in the buffer.
func buffered(n int) []Task {
	tasks := make([]Task, 0, n+1)
	tasks = append(tasks, restoreTask("run"))
	for i := range n {
		tasks = append(tasks, restoreTask(fmt.Sprintf("buf%d", i)))
	}
	return tasks
}

func restoreTask(id string) Task {
	return Task{ID: id, Action: ActionRestore, Content: json.RawMessage(`{"id":"q1"}`)}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStart_StopAcksBufferedTasksWithAnError(t *testing.T) {
	f := &fakeConsole{
		queue: []Task{restoreTask("run"), restoreTask("buf1"), restoreTask("buf2")},
		acks:  map[string]string{},
	}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	conn := &blockingConnector{entered: make(chan struct{}), release: make(chan struct{})}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		newTestClient(srv).Start(ctx, conn)
	}()

	<-conn.entered
	// The polling goroutine keeps fetching while the first task runs: both
	// others end up buffered (served) and a fourth poll is held open.
	eventually(t, "the buffered tasks to be fetched", func() bool { return f.pollCount() >= 4 })
	cancel()
	close(conn.release)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return after the stop")
	}

	if e, ok := f.ackOf("run"); !ok || e != "" {
		t.Errorf("task that ran: ack = (%q, %v), want an ack without error", e, ok)
	}
	for _, id := range []string{"buf1", "buf2"} {
		if e, ok := f.ackOf(id); !ok || e != stoppedTaskError {
			t.Errorf("task %s: ack = (%q, %v), want %q", id, e, ok, stoppedTaskError)
		}
	}
}

func TestStart_StopStaysBoundedWhenConsoleDoesNotAnswer(t *testing.T) {
	prev := ackTimeout
	ackTimeout = 500 * time.Millisecond
	t.Cleanup(func() { ackTimeout = prev })

	f := &fakeConsole{
		queue:   buffered(9),
		acks:    map[string]string{},
		hangAck: make(chan struct{}),
	}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	defer close(f.hangAck)
	conn := &blockingConnector{entered: make(chan struct{}), release: make(chan struct{})}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		newTestClient(srv).Start(ctx, conn)
	}()

	<-conn.entered
	eventually(t, "the buffered tasks to be fetched", func() bool { return f.pollCount() >= 11 })
	cancel()
	close(conn.release)
	stopped := time.Now()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return although the console does not answer")
	}
	// One ackTimeout for the task that ran and one for the whole drain; a
	// timeout per buffered task would take 9 times more.
	if took := time.Since(stopped); took > 4*ackTimeout {
		t.Errorf("stop took %s, want less than %s", took, 4*ackTimeout)
	}
}

func TestFailBufferedTasks_acksEveryTaskAsStoppedWithoutRunningAny(t *testing.T) {
	f := &fakeConsole{acks: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tasks := make(chan Task, 3)
	for _, id := range []string{"b1", "b2", "b3"} {
		tasks <- restoreTask(id)
	}
	close(tasks)
	first := restoreTask("first")

	newTestClient(srv).failBufferedTasks(ctx, &first, tasks)

	for _, id := range []string{"first", "b1", "b2", "b3"} {
		if e, ok := f.ackOf(id); !ok || e != stoppedTaskError {
			t.Errorf("task %s: ack = (%q, %v), want %q", id, e, ok, stoppedTaskError)
		}
	}
}

func TestStart_ContextCancelledBeforeStartFetchesAndRunsNothing(t *testing.T) {
	f := &fakeConsole{queue: buffered(3), acks: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	conn := &blockingConnector{entered: make(chan struct{}), release: make(chan struct{})}
	close(conn.release)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		newTestClient(srv).Start(ctx, conn)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Start did not return with a cancelled context")
	}
	select {
	case <-conn.entered:
		t.Error("a task ran although the context was cancelled before Start")
	default:
	}
	if n := f.pollCount(); n != 0 {
		t.Errorf("%d polls although the context was cancelled before Start", n)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.acks) != 0 {
		t.Errorf("acks = %v, want none: nothing was fetched", f.acks)
	}
}
