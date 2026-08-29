package httpserver

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
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
	// The milestone entity is deliberately absent above: its calendar is replaced
	// wholesale, so a delete is never emitted for it.
}

func TestMilestoneSyncActionRoundTripsTheCalendar(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(map[string]any{"milestones": []directory.Milestone{
		{Name: "一籌", Date: "2026-08-29", Kind: directory.MilestoneOrganizing},
		{Name: "年會", Date: "2027-03-13", Kind: directory.MilestoneConference},
	}})
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := mapSyncAction(board.SyncAction{
		Entity: "milestone", SyncID: "12", EntityID: "directory", Operation: "upsert",
		OccurredAt: time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC), Payload: payload,
	})
	if err != nil {
		t.Fatalf("mapSyncAction() error = %v", err)
	}
	want := []directoryMilestoneResponse{
		{Name: "一籌", Date: "2026-08-29", Kind: "organizing"},
		{Name: "年會", Date: "2027-03-13", Kind: "conference"},
	}
	if mapped.Entity != "milestone" || !reflect.DeepEqual(mapped.Milestones, want) {
		t.Fatalf("mapped = %#v, want milestones %#v", mapped, want)
	}
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("marshal milestone action: %v", err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("decode milestone action: %v", err)
	}
	if _, exists := wire["milestones"]; !exists {
		t.Fatalf("milestone action = %s, want a milestones array", encoded)
	}
}
