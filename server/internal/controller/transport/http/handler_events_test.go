package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type revisionEventsFake struct {
	updates chan string
}

func (f *revisionEventsFake) Revision(context.Context) (string, error) { return "8", nil }
func (f *revisionEventsFake) SubscribeRevisions() (<-chan string, func()) {
	return f.updates, func() {}
}

func TestBootstrapEventsSendsCurrentAndChangedRevision(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events/bootstrap", nil).WithContext(ctx)
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
