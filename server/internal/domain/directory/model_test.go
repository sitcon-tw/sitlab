package directory

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 14, 8, 0, 0, 0, time.UTC)
	file := File{Version: 1, Teams: []TeamConfig{
		{Key: "administration", Name: "行政組", TitlePrefix: "[行政組]", GitLabLabel: "組別::行政", Active: true, Members: []string{"alice", "missing"}, Leaders: []string{"alice"}},
		{Key: "development", Name: "開發組", TitlePrefix: "[開發組]", GitLabLabel: "組別::開發", Active: true, Members: []string{"ALICE", "bob"}},
	}, Milestones: []MilestoneConfig{
		{Name: "一籌", Date: "2026-08-29", Kind: MilestoneOrganizing},
		{Name: " 站立會議 ", Date: "2026-08-22", Kind: MilestoneStandup},
	}}
	members := []GitLabMember{
		{GitLabUserID: 2, Username: "bob", DisplayName: "Bob", State: MemberActive},
		{GitLabUserID: 1, Username: "alice", DisplayName: "Alice", State: MemberActive},
		{GitLabUserID: 3, Username: "ungrouped", DisplayName: "Zed", State: MemberBlocked},
	}

	snapshot, missing, err := Normalize(file, members, "abc123", now)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got, want := snapshot.Teams[0].MemberGitLabUserIDs, []int64{1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("administration members = %v, want %v", got, want)
	}
	if got, want := snapshot.Teams[0].LeaderGitLabUserIDs, []int64{1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("administration leaders = %v, want %v", got, want)
	}
	if got, want := snapshot.Members[0].TeamKeys, []string{"administration", "development"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("alice teams = %v, want %v", got, want)
	}
	if got, want := missing, []MissingMember{{TeamKey: "administration", Username: "missing"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing = %#v, want %#v", got, want)
	}
	if !snapshot.IsAssignable(1) || snapshot.IsAssignable(3) {
		t.Fatal("assignability did not respect GitLab state")
	}
	if snapshot.Members[2].Username != "ungrouped" || len(snapshot.Members[2].TeamKeys) != 0 {
		t.Fatalf("ungrouped member = %#v", snapshot.Members[2])
	}
	wantMilestones := []Milestone{
		{Name: "站立會議", Date: "2026-08-22", Kind: MilestoneStandup},
		{Name: "一籌", Date: "2026-08-29", Kind: MilestoneOrganizing},
	}
	if !reflect.DeepEqual(snapshot.Milestones, wantMilestones) {
		t.Fatalf("milestones = %#v, want %#v", snapshot.Milestones, wantMilestones)
	}
}

func TestNormalizeRejectsInvalidDirectory(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file File
		want error
	}{
		{name: "version", file: File{Version: 2}, want: ErrUnsupportedVersion},
		{name: "empty team", file: File{Version: 1, Teams: []TeamConfig{{Key: "development"}}}, want: ErrInvalidTeam},
		{name: "duplicate team", file: File{Version: 1, Teams: []TeamConfig{
			{Key: "development", Name: "開發組", TitlePrefix: "[開發組]", GitLabLabel: "組別::開發"},
			{Key: "development", Name: "開發二組", TitlePrefix: "[開發二組]", GitLabLabel: "組別::開發二"},
		}}, want: ErrDuplicateTeam},
		{name: "milestone bad date", file: File{Version: 1, Milestones: []MilestoneConfig{
			{Name: "一籌", Date: "2026-13-01", Kind: MilestoneOrganizing},
		}}, want: ErrInvalidMilestone},
		{name: "milestone slash date", file: File{Version: 1, Milestones: []MilestoneConfig{
			{Name: "一籌", Date: "08/29/2026", Kind: MilestoneOrganizing},
		}}, want: ErrInvalidMilestone},
		{name: "milestone empty name", file: File{Version: 1, Milestones: []MilestoneConfig{
			{Name: "  ", Date: "2026-08-29", Kind: MilestoneOrganizing},
		}}, want: ErrInvalidMilestone},
		{name: "milestone unknown kind", file: File{Version: 1, Milestones: []MilestoneConfig{
			{Name: "一籌", Date: "2026-08-29", Kind: "party"},
		}}, want: ErrInvalidMilestone},
		{name: "duplicate milestone", file: File{Version: 1, Milestones: []MilestoneConfig{
			{Name: "一籌", Date: "2026-08-29", Kind: MilestoneOrganizing},
			{Name: "一籌", Date: "2026-08-29", Kind: MilestoneStandup},
		}}, want: ErrDuplicateMilestone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Normalize(tt.file, nil, "revision", time.Now())
			if !errors.Is(err, tt.want) {
				t.Fatalf("Normalize() error = %v, want %v", err, tt.want)
			}
		})
	}
}
