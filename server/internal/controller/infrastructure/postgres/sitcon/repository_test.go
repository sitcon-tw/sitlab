package sitcon

import (
	"slices"
	"testing"
)

func TestPlaceIssueIID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		order          []int64
		issueIID       int64
		targetPosition int32
		wantOrder      []int64
		wantPosition   int32
	}{
		{name: "move first down", order: []int64{10, 20, 30}, issueIID: 10, targetPosition: 1, wantOrder: []int64{20, 10, 30}, wantPosition: 1},
		{name: "move last up", order: []int64{10, 20, 30}, issueIID: 30, targetPosition: 1, wantOrder: []int64{10, 30, 20}, wantPosition: 1},
		{name: "insert from another list", order: []int64{10, 20}, issueIID: 30, targetPosition: 1, wantOrder: []int64{10, 30, 20}, wantPosition: 1},
		{name: "clamp after last", order: []int64{10, 20}, issueIID: 30, targetPosition: 99, wantOrder: []int64{10, 20, 30}, wantPosition: 2},
		{name: "clamp before first", order: []int64{10, 20}, issueIID: 30, targetPosition: -1, wantOrder: []int64{30, 10, 20}, wantPosition: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotOrder, gotPosition := placeIssueIID(tt.order, tt.issueIID, tt.targetPosition)
			if !slices.Equal(gotOrder, tt.wantOrder) || gotPosition != tt.wantPosition {
				t.Fatalf("placeIssueIID() = %v, %d, want %v, %d", gotOrder, gotPosition, tt.wantOrder, tt.wantPosition)
			}
		})
	}
}

func TestRemoveIssueIID(t *testing.T) {
	t.Parallel()
	got := removeIssueIID([]int64{10, 20, 30}, 20)
	if want := []int64{10, 30}; !slices.Equal(got, want) {
		t.Fatalf("removeIssueIID() = %v, want %v", got, want)
	}
}
