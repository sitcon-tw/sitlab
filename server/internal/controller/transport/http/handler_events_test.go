package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
)

type revisionEventsFake struct {
	updates chan string
}

func (f *revisionEventsFake) Revision(context.Context) (string, error) { return "8", nil }
func (f *revisionEventsFake) SubscribeRevisions() (<-chan string, func()) {
	return f.updates, func() {}
}

type deltaSyncFake struct {
	since []string
	err   error
}

func (*deltaSyncFake) RequestRefresh() time.Time { return time.Time{} }
func (*deltaSyncFake) EnqueueWebhook(context.Context, board.WebhookDelivery) (bool, error) {
	return false, nil
}
func (f *deltaSyncFake) Delta(_ context.Context, query appsync.DeltaQuery) (board.SyncDelta, error) {
	f.since = append(f.since, query.Since)
	if f.err != nil {
		return board.SyncDelta{}, f.err
	}
	return board.SyncDelta{Checkpoint: "42"}, nil
}

func TestSyncEventsPrefersLastEventIDOverTheQueryString(t *testing.T) {
	// A browser reconnecting on its own cannot change the URL it was opened with, so
	// Last-Event-ID is the only way it can report where it left off. Reading the stale
	// query string instead looks fine in development and silently drops changes in
	// production on every network blip.
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/events/sync?since=10", nil)
	request.Header.Set("Last-Event-ID", "37")
	recorder := httptest.NewRecorder()
	events := &revisionEventsFake{updates: make(chan string, 1)}
	sync := &deltaSyncFake{}
	done := make(chan struct{})
	go func() {
		handler{events: events, sync: sync}.syncEvents(recorder, request)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sync event handler did not stop after cancellation")
	}
	if len(sync.since) == 0 {
		t.Fatal("stream never asked for a delta")
	}
	if sync.since[0] != "37" {
		t.Fatalf("stream resumed from %q, want the Last-Event-ID checkpoint 37", sync.since[0])
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: sync") || !strings.Contains(body, `"checkpoint":"42"`) {
		t.Fatalf("stream body = %q, want a sync frame carrying the checkpoint", body)
	}
	if !strings.Contains(body, "id: 42") {
		t.Fatalf("stream body = %q, want the checkpoint as the SSE id so reconnects resume from it", body)
	}
}

func TestSyncEventsTellsAnUnknownCheckpointToReload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/events/sync", nil)
	recorder := httptest.NewRecorder()
	events := &revisionEventsFake{updates: make(chan string, 1)}
	sync := &deltaSyncFake{}
	done := make(chan struct{})
	go func() {
		handler{events: events, sync: sync}.syncEvents(recorder, request)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if len(sync.since) != 0 {
		t.Fatalf("stream asked for a delta with no checkpoint: %v", sync.since)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: reset") || !strings.Contains(body, `"reason":"checkpointUnknown"`) || !strings.Contains(body, `"checkpoint":"8"`) {
		t.Fatalf("stream body = %q, want a reset carrying the current checkpoint", body)
	}
}

func TestSyncEventsEndsATooOldStreamWithTheCurrentCheckpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events/sync?since=3", nil)
	recorder := httptest.NewRecorder()
	events := &revisionEventsFake{updates: make(chan string, 1)}
	sync := &deltaSyncFake{err: board.ErrCheckpointTooOld}

	handler{events: events, sync: sync}.syncEvents(recorder, request)

	body := recorder.Body.String()
	if !strings.Contains(body, "event: reset") || !strings.Contains(body, `"reason":"checkpointTooOld"`) || !strings.Contains(body, `"checkpoint":"8"`) {
		t.Fatalf("stream body = %q, want a terminal reset carrying the current checkpoint", body)
	}
	if strings.Contains(body, "event: heartbeat") {
		t.Fatalf("stream body = %q, reset stream must terminate before an empty heartbeat", body)
	}
}

func TestBootstrapEventsSendsCurrentAndChangedRevision(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/events/bootstrap", nil)
	recorder := httptest.NewRecorder()
	events := &revisionEventsFake{updates: make(chan string, 1)}
	done := make(chan struct{})
	go func() {
		handler{events: events}.bootstrapEvents(recorder, request)
		close(done)
	}()
	events.updates <- "9"
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event handler did not stop after cancellation")
	}
	body := recorder.Body.String()
	if recorder.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(body, "id: 8") || !strings.Contains(body, "id: 9") {
		t.Fatalf("headers=%v body=%q", recorder.Header(), body)
	}
}
