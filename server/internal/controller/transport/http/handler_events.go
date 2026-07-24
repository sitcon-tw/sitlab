package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
)

const eventHeartbeatInterval = 15 * time.Second

func (h handler) bootstrapEvents(w http.ResponseWriter, r *http.Request) {
	if h.events == nil {
		writeError(w, r, apperror.Unavailable("realtime events are unavailable"))
		return
	}
	updates, unsubscribe := h.events.SubscribeRevisions()
	defer unsubscribe()
	if h.metrics != nil {
		h.metrics.SSEConnected()
		defer h.metrics.SSEDisconnected()
	}
	revision, err := h.events.Revision(r.Context())
	if err != nil {
		writeError(w, r, apperror.Unavailable("bootstrap revision is unavailable"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	if err := writeBootstrapEvent(w, revision); err != nil {
		return
	}
	if h.metrics != nil {
		h.metrics.SSEEvent()
	}
	if err := controller.Flush(); err != nil {
		return
	}
	heartbeat := time.NewTicker(eventHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case revision := <-updates:
			if err := writeBootstrapEvent(w, revision); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
			if h.metrics != nil {
				h.metrics.SSEEvent()
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func writeBootstrapEvent(w http.ResponseWriter, revision string) error {
	payload, err := json.Marshal(map[string]string{"revision": revision})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: bootstrap\ndata: %s\n\n", revision, payload)
	return err
}
