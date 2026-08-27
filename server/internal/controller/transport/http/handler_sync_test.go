package httpserver

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSyncActionDeletesCarryTheRequiredNullPayload(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		entity     string
		payloadKey string
	}{
		{entity: "card", payloadKey: "card"},
		{entity: "list", payloadKey: "list"},
		{entity: "team", payloadKey: "team"},
		{entity: "member", payloadKey: "member"},
	} {
		t.Run(test.entity, func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(syncActionResponse{
				Entity: test.entity, SyncID: "12", EntityID: "fixture",
				Operation: "delete", OccurredAt: time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("marshal delete action: %v", err)
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatalf("decode delete action: %v", err)
			}
			value, exists := payload[test.payloadKey]
			if !exists || string(value) != "null" {
				t.Fatalf("delete action = %s, want required %q: null", encoded, test.payloadKey)
			}
		})
	}
}
