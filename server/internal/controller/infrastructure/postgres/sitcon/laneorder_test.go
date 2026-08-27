package sitcon

import (
	"slices"
	"testing"
)

func TestLaneOrdersInsertAt(t *testing.T) {
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
			orders := laneOrders{"doing": slices.Clone(tt.order)}
			gotPosition := orders.insertAt("doing", tt.issueIID, tt.targetPosition)
			if !slices.Equal(orders["doing"], tt.wantOrder) || gotPosition != tt.wantPosition {
				t.Fatalf("insertAt() = %v, %d, want %v, %d", orders["doing"], gotPosition, tt.wantOrder, tt.wantPosition)
			}
		})
	}
}

func TestLaneOrdersRemove(t *testing.T) {
	t.Parallel()
	orders := laneOrders{"doing": {10, 20, 30}}
	orders.remove("doing", 20)
	if want := []int64{10, 30}; !slices.Equal(orders["doing"], want) {
		t.Fatalf("remove() = %v, want %v", orders["doing"], want)
	}
	orders.remove("doing", 999)
	if want := []int64{10, 30}; !slices.Equal(orders["doing"], want) {
		t.Fatalf("remove() of an absent card = %v, want %v", orders["doing"], want)
	}
	orders.remove("missing-lane", 10)
}

func TestLaneOrdersPrependPutsNewCardsOnTop(t *testing.T) {
	t.Parallel()
	orders := laneOrders{"doing": {10, 20}}
	orders.prepend("doing", 30, 40)
	if want := []int64{30, 40, 10, 20}; !slices.Equal(orders["doing"], want) {
		t.Fatalf("prepend() = %v, want %v", orders["doing"], want)
	}
	orders.prepend("inbox", 50)
	if want := []int64{50}; !slices.Equal(orders["inbox"], want) {
		t.Fatalf("prepend() into an empty lane = %v, want %v", orders["inbox"], want)
	}
	orders.prepend("doing")
	if want := []int64{30, 40, 10, 20}; !slices.Equal(orders["doing"], want) {
		t.Fatalf("prepend() of nothing = %v, want %v", orders["doing"], want)
	}
}

func TestLaneOrdersCloneIsIndependent(t *testing.T) {
	t.Parallel()
	orders := laneOrders{"doing": {10, 20}}
	before := orders.clone()
	orders.insertAt("doing", 30, 0)
	if want := []int64{10, 20}; !slices.Equal(before["doing"], want) {
		t.Fatalf("clone() shared its backing array: %v, want %v", before["doing"], want)
	}
}

func TestLaneOrdersPositions(t *testing.T) {
	t.Parallel()
	positions := laneOrders{"doing": {30, 10}, "inbox": {20}}.positions()
	want := map[int64]int32{30: 0, 10: 1, 20: 0}
	for issueIID, wantPosition := range want {
		if positions[issueIID] != wantPosition {
			t.Fatalf("positions()[%d] = %d, want %d", issueIID, positions[issueIID], wantPosition)
		}
	}
	if len(positions) != len(want) {
		t.Fatalf("positions() = %v, want %d entries", positions, len(want))
	}
}
