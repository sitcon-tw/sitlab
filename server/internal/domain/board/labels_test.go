package board_test

import (
	"testing"

	"example.com/project-template/internal/domain/board"
)

// This table mirrors web/src/features/board/labels.test.ts so drift between the
// two implementations of the rule shows up as a diff on both sides.
func TestReservedLabel(t *testing.T) {
	teamLabels := []string{"Team::開發組", "Team::行政組"}
	cases := []struct {
		name     string
		label    string
		reserved bool
		reason   string
	}{
		{name: "configured team label", label: "Team::開發組", reserved: true, reason: "owned by board-directory.yml"},
		{name: "unconfigured team prefix", label: "Team::新組", reserved: true, reason: "the prefix alone is reserved"},
		{name: "lifecycle label", label: "Status::Inbox", reserved: true, reason: "the board owns lifecycle"},
		{name: "legacy workflow label", label: "To Do", reserved: true, reason: "legacy workflow"},
		{name: "legacy team label", label: "組別::開發", reserved: true, reason: "legacy team naming"},
		{name: "blank", label: "   ", reserved: true, reason: "never a valid name"},
		{name: "ordinary label", label: "Backend", reserved: false},
		{name: "namespaced but unreserved", label: "Priority::High", reserved: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := board.ReservedLabel(testCase.label, teamLabels); got != testCase.reserved {
				t.Fatalf("ReservedLabel(%q) = %v, want %v (%s)", testCase.label, got, testCase.reserved, testCase.reason)
			}
		})
	}
}

func TestDeprecatedLabel(t *testing.T) {
	for _, label := range []string{"Status::Doing", "Wating", "Doing", "組別::總召"} {
		if !board.DeprecatedLabel(label) {
			t.Fatalf("DeprecatedLabel(%q) = false, want true", label)
		}
	}
	for _, label := range []string{"Backend", "Priority::High", "Team::開發組"} {
		if board.DeprecatedLabel(label) {
			t.Fatalf("DeprecatedLabel(%q) = true, want false", label)
		}
	}
}
