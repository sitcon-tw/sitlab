package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
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

// syncEvents streams recorded changes rather than a bare "something changed" nudge.
//
// Delivery here is opportunistic on purpose. Every frame carries the checkpoint it
// reached, so a client that notices a gap falls back to GET /sync and catches up. That
// is what lets the in-process fan-out keep dropping wake-ups when a subscriber is slow,
// lets the browser ignore frames mid-drag, and makes a dropped connection harmless. If
// frames had to arrive, none of those would be safe.
func (h handler) syncEvents(w http.ResponseWriter, r *http.Request) {
	if h.events == nil || h.sync == nil {
		writeError(w, r, apperror.Unavailable("realtime events are unavailable"))
		return
	}
	updates, unsubscribe := h.events.SubscribeRevisions()
	defer unsubscribe()
	if h.metrics != nil {
		h.metrics.SSEConnected()
		defer h.metrics.SSEDisconnected()
	}
	claims := claimsFromContext(r.Context())
	// A browser reconnecting on its own cannot change the URL, so Last-Event-ID is the
	// only channel it has to say where it left off. It has to win over the query string.
	checkpoint := r.Header.Get("Last-Event-ID")
	if checkpoint == "" {
		checkpoint = r.URL.Query().Get("since")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)

	reset := func(reason string) error {
		current, err := h.events.Revision(r.Context())
		if err != nil {
			return err
		}
		if err := writeResetEvent(w, reason, current); err != nil {
			return err
		}
		return controller.Flush()
	}
	// drain returns reset=true after writing a terminal reset frame. The client must
	// load a full bootstrap and open a new stream from that checkpoint; keeping this
	// stream alive would only produce heartbeats with an unusable cursor.
	drain := func() (resetSent bool, err error) {
		if checkpoint == "" {
			return true, reset("checkpointUnknown")
		}
		for {
			previous := checkpoint
			delta, deltaErr := h.sync.Delta(r.Context(), appsync.DeltaQuery{Since: checkpoint, AudienceUserID: claims.UserID})
			if errors.Is(deltaErr, board.ErrCheckpointTooOld) {
				return true, reset("checkpointTooOld")
			}
			if deltaErr != nil {
				return false, deltaErr
			}
			response, mapErr := mapSyncDelta(delta)
			if mapErr != nil {
				return false, mapErr
			}
			checkpoint = delta.Checkpoint
			// A heartbeat calls drain as a periodic authoritative catch-up. Do not emit
			// an empty sync frame when the checkpoint did not move.
			if checkpoint != previous || len(delta.Actions) > 0 {
				if err := writeSyncEvent(w, response); err != nil {
					return false, err
				}
				if err := controller.Flush(); err != nil {
					return false, err
				}
				if h.metrics != nil {
					h.metrics.SSEEvent()
				}
			}
			if !delta.HasMore {
				return false, nil
			}
		}
	}

	resetSent, err := drain()
	if err != nil || resetSent {
		return
	}
	// Flush headers even when the initial catch-up was empty so EventSource reaches
	// OPEN without waiting for the first heartbeat.
	if err := controller.Flush(); err != nil {
		return
	}
	heartbeat := time.NewTicker(eventHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-updates:
			resetSent, err = drain()
			if err != nil || resetSent {
				return
			}
		case <-heartbeat.C:
			// Query the authoritative log before each heartbeat. This catches changes
			// even if a LISTEN/NOTIFY wake-up was lost while the listener reconnected.
			resetSent, err = drain()
			if err != nil || resetSent {
				return
			}
			if err := writeHeartbeatEvent(w, checkpoint); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func writeSyncEvent(w http.ResponseWriter, delta syncDeltaResponse) error {
	payload, err := json.Marshal(delta)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: sync\ndata: %s\n\n", delta.Checkpoint, payload)
	return err
}

func writeResetEvent(w http.ResponseWriter, reason, checkpoint string) error {
	payload, err := json.Marshal(map[string]string{"reason": reason, "checkpoint": checkpoint})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: reset\ndata: %s\n\n", payload)
	return err
}

func writeHeartbeatEvent(w http.ResponseWriter, checkpoint string) error {
	payload, err := json.Marshal(map[string]string{"checkpoint": checkpoint})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: heartbeat\ndata: %s\n\n", payload)
	return err
}
