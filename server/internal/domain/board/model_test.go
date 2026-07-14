package board

import (
	"errors"
	"testing"
	"time"
)

type fakeDirectory struct {
	teams       map[string]bool
	assignable  map[int64]bool
	memberships map[string]map[int64]bool
}

func (f fakeDirectory) TeamExists(teamKey string) bool { return f.teams[teamKey] }
func (f fakeDirectory) IsAssignable(id int64) bool     { return f.assignable[id] }
func (f fakeDirectory) IsMemberOf(id int64, team string) bool {
	return f.memberships[team][id]
}

func TestDefaultAssignee(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, selected, primary string
		want                    bool
	}{
		{name: "same team", selected: "development", primary: "development", want: true},
		{name: "different team", selected: "design", primary: "development", want: false},
		{name: "no preference", selected: "development", primary: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DefaultAssignee(tt.selected, tt.primary, 123)
			if (got != nil) != tt.want {
				t.Fatalf("DefaultAssignee() = %v, want assigned %v", got, tt.want)
			}
		})
	}
}

func TestReconcileAssignee(t *testing.T) {
	t.Parallel()
	directory := fakeDirectory{
		teams:      map[string]bool{"development": true, "design": true},
		assignable: map[int64]bool{1: true, 2: false},
		memberships: map[string]map[int64]bool{
			"development": {1: true},
			"design":      {},
		},
	}
	one, two := int64(1), int64(2)
	tests := []struct {
		name    string
		team    string
		current *int64
		cleared bool
		wantErr error
	}{
		{name: "preserve active member", team: "development", current: &one},
		{name: "clear member from other team", team: "design", current: &one, cleared: true},
		{name: "clear inactive member", team: "development", current: &two, cleared: true},
		{name: "leave unassigned", team: "design"},
		{name: "unknown team", team: "unknown", current: &one, wantErr: ErrTeamNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, cleared, err := ReconcileAssignee(directory, tt.team, tt.current)
			if !errors.Is(err, tt.wantErr) || cleared != tt.cleared {
				t.Fatalf("ReconcileAssignee() = %v, %v, %v", got, cleared, err)
			}
			if tt.cleared && got != nil {
				t.Fatalf("ReconcileAssignee() kept %d", *got)
			}
		})
	}
}

func TestComposeGitLabTitle(t *testing.T) {
	t.Parallel()
	tests := []struct{ prefix, title, want string }{
		{prefix: "[開發組]", title: "修正  報名系統流程", want: "[開發組] 修正 報名系統流程"},
		{prefix: "[開發組]", title: "[開發組] 修正流程", want: "[開發組] 修正流程"},
		{title: "修正流程", want: "修正流程"},
	}
	for _, tt := range tests {
		if got := ComposeGitLabTitle(tt.prefix, tt.title); got != tt.want {
			t.Errorf("ComposeGitLabTitle(%q, %q) = %q, want %q", tt.prefix, tt.title, got, tt.want)
		}
	}
}

func TestDefaultDueDateUsesTaipei(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 14, 18, 0, 0, 0, time.UTC)
	if got, want := DefaultDueDate(now), "2026-07-22"; got != want {
		t.Fatalf("DefaultDueDate() = %q, want %q", got, want)
	}
}
