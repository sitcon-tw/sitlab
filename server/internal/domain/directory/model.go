package directory

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type MemberState string

const (
	MemberActive      MemberState = "active"
	MemberBlocked     MemberState = "blocked"
	MemberDeactivated MemberState = "deactivated"
)

type MilestoneKind string

const (
	MilestoneOrganizing MilestoneKind = "organizing"
	MilestoneStandup    MilestoneKind = "standup"
	MilestoneConference MilestoneKind = "conference"
)

var (
	ErrUnsupportedVersion  = errors.New("unsupported directory version")
	ErrInvalidTeam         = errors.New("invalid directory team")
	ErrDuplicateTeam       = errors.New("duplicate directory team")
	ErrDuplicateUsername   = errors.New("duplicate GitLab username")
	ErrInvalidMilestone    = errors.New("invalid directory milestone")
	ErrDuplicateMilestone  = errors.New("duplicate directory milestone")
	ErrSnapshotNotFound    = errors.New("directory snapshot not found")
	ErrPreferencesNotFound = errors.New("user preferences not found")
)

type File struct {
	Version    int
	Teams      []TeamConfig
	Milestones []MilestoneConfig
}

type TeamConfig struct {
	Key         string
	Name        string
	TitlePrefix string
	GitLabLabel string
	Active      bool
	Members     []string
	Leaders     []string
}

type MilestoneConfig struct {
	Name string
	Date string
	Kind MilestoneKind
}

type Milestone struct {
	Name string
	Date string
	Kind MilestoneKind
}

type GitLabMember struct {
	GitLabUserID int64
	Username     string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
	AccessLevel  int32
	State        MemberState
}

type Member struct {
	GitLabUserID int64
	Username     string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
	AccessLevel  int32
	State        MemberState
	TeamKeys     []string
}

func (m Member) Assignable() bool {
	return m.GitLabUserID > 0 && m.State == MemberActive
}

type Team struct {
	Key                      string
	Name                     string
	TitlePrefix              string
	GitLabLabel              string
	Active                   bool
	SortOrder                int32
	MemberGitLabUserIDs      []int64
	LeaderGitLabUserIDs      []int64
	DirectoryMemberUsernames []string
	DirectoryLeaderUsernames []string
}

type MissingMember struct {
	TeamKey  string
	Username string
}

type Snapshot struct {
	Teams          []Team
	Members        []Member
	Milestones     []Milestone
	SourceRevision string
	SyncedAt       time.Time
}

type Preferences struct {
	DefaultTeamKey    *string
	ConfirmedAt       *time.Time
	DirectoryTeamKeys []string
}

func Normalize(file File, gitLabMembers []GitLabMember, sourceRevision string, syncedAt time.Time) (Snapshot, []MissingMember, error) {
	if file.Version != 1 {
		return Snapshot{}, nil, fmt.Errorf("%w: %d", ErrUnsupportedVersion, file.Version)
	}

	membersByUsername := make(map[string]GitLabMember, len(gitLabMembers))
	for _, member := range gitLabMembers {
		member.Username = strings.TrimSpace(member.Username)
		member.DisplayName = strings.TrimSpace(member.DisplayName)
		key := strings.ToLower(member.Username)
		if member.GitLabUserID <= 0 || key == "" || member.DisplayName == "" || !validMemberState(member.State) {
			return Snapshot{}, nil, fmt.Errorf("invalid GitLab member %q", member.Username)
		}
		if _, exists := membersByUsername[key]; exists {
			return Snapshot{}, nil, fmt.Errorf("%w: %s", ErrDuplicateUsername, member.Username)
		}
		membersByUsername[key] = member
	}

	teams := make([]Team, 0, len(file.Teams))
	teamKeys := make(map[string]struct{}, len(file.Teams))
	memberTeams := make(map[int64][]string, len(gitLabMembers))
	missing := make([]MissingMember, 0)
	for index, config := range file.Teams {
		config.Key = strings.TrimSpace(config.Key)
		config.Name = strings.TrimSpace(config.Name)
		config.TitlePrefix = strings.TrimSpace(config.TitlePrefix)
		config.GitLabLabel = strings.TrimSpace(config.GitLabLabel)
		if config.Key == "" || config.Name == "" || config.TitlePrefix == "" || config.GitLabLabel == "" {
			return Snapshot{}, nil, fmt.Errorf("%w at index %d", ErrInvalidTeam, index)
		}
		if _, exists := teamKeys[config.Key]; exists {
			return Snapshot{}, nil, fmt.Errorf("%w: %s", ErrDuplicateTeam, config.Key)
		}
		teamKeys[config.Key] = struct{}{}

		team := Team{
			Key: config.Key, Name: config.Name, TitlePrefix: config.TitlePrefix,
			GitLabLabel: config.GitLabLabel, Active: config.Active, SortOrder: int32(index),
		}
		seenMember := make(map[int64]struct{}, len(config.Members))
		for _, rawUsername := range config.Members {
			username := strings.TrimSpace(rawUsername)
			member, exists := membersByUsername[strings.ToLower(username)]
			if !exists {
				missing = append(missing, MissingMember{TeamKey: config.Key, Username: username})
				continue
			}
			if _, exists := seenMember[member.GitLabUserID]; exists {
				continue
			}
			seenMember[member.GitLabUserID] = struct{}{}
			team.MemberGitLabUserIDs = append(team.MemberGitLabUserIDs, member.GitLabUserID)
			team.DirectoryMemberUsernames = append(team.DirectoryMemberUsernames, member.Username)
			memberTeams[member.GitLabUserID] = append(memberTeams[member.GitLabUserID], config.Key)
		}
		seenLeader := make(map[int64]struct{}, len(config.Leaders))
		for _, rawUsername := range config.Leaders {
			username := strings.TrimSpace(rawUsername)
			member, exists := membersByUsername[strings.ToLower(username)]
			if !exists {
				missing = append(missing, MissingMember{TeamKey: config.Key, Username: username})
				continue
			}
			if _, exists := seenLeader[member.GitLabUserID]; exists {
				continue
			}
			seenLeader[member.GitLabUserID] = struct{}{}
			team.LeaderGitLabUserIDs = append(team.LeaderGitLabUserIDs, member.GitLabUserID)
			team.DirectoryLeaderUsernames = append(team.DirectoryLeaderUsernames, member.Username)
			if _, exists := seenMember[member.GitLabUserID]; !exists {
				seenMember[member.GitLabUserID] = struct{}{}
				team.MemberGitLabUserIDs = append(team.MemberGitLabUserIDs, member.GitLabUserID)
				team.DirectoryMemberUsernames = append(team.DirectoryMemberUsernames, member.Username)
				memberTeams[member.GitLabUserID] = append(memberTeams[member.GitLabUserID], config.Key)
			}
		}
		teams = append(teams, team)
	}

	milestones := make([]Milestone, 0, len(file.Milestones))
	milestoneKeys := make(map[string]struct{}, len(file.Milestones))
	for index, config := range file.Milestones {
		name := strings.TrimSpace(config.Name)
		if name == "" {
			return Snapshot{}, nil, fmt.Errorf("%w at index %d: empty name", ErrInvalidMilestone, index)
		}
		parsed, err := time.Parse(time.DateOnly, config.Date)
		if err != nil {
			return Snapshot{}, nil, fmt.Errorf("%w at index %d: date %q", ErrInvalidMilestone, index, config.Date)
		}
		if !validMilestoneKind(config.Kind) {
			return Snapshot{}, nil, fmt.Errorf("%w at index %d: kind %q", ErrInvalidMilestone, index, config.Kind)
		}
		milestone := Milestone{Name: name, Date: parsed.Format(time.DateOnly), Kind: config.Kind}
		duplicateKey := milestone.Date + "\x00" + milestone.Name
		if _, exists := milestoneKeys[duplicateKey]; exists {
			return Snapshot{}, nil, fmt.Errorf("%w: %s %s", ErrDuplicateMilestone, milestone.Date, milestone.Name)
		}
		milestoneKeys[duplicateKey] = struct{}{}
		milestones = append(milestones, milestone)
	}
	sort.SliceStable(milestones, func(i, j int) bool {
		if milestones[i].Date == milestones[j].Date {
			return milestones[i].Name < milestones[j].Name
		}
		return milestones[i].Date < milestones[j].Date
	})

	members := make([]Member, 0, len(gitLabMembers))
	for _, source := range gitLabMembers {
		teamsForMember := append([]string(nil), memberTeams[source.GitLabUserID]...)
		members = append(members, Member{
			GitLabUserID: source.GitLabUserID, Username: strings.TrimSpace(source.Username),
			DisplayName: strings.TrimSpace(source.DisplayName), AvatarURL: strings.TrimSpace(source.AvatarURL),
			ProfileURL: strings.TrimSpace(source.ProfileURL), AccessLevel: source.AccessLevel,
			State: source.State, TeamKeys: teamsForMember,
		})
	}
	sort.SliceStable(members, func(i, j int) bool {
		left, right := strings.ToLower(members[i].DisplayName), strings.ToLower(members[j].DisplayName)
		if left == right {
			return strings.ToLower(members[i].Username) < strings.ToLower(members[j].Username)
		}
		return left < right
	})

	return Snapshot{Teams: teams, Members: members, Milestones: milestones, SourceRevision: sourceRevision, SyncedAt: syncedAt.UTC()}, missing, nil
}

func (s Snapshot) TeamExists(teamKey string) bool {
	for _, team := range s.Teams {
		if team.Key == teamKey && team.Active {
			return true
		}
	}
	return false
}

func (s Snapshot) IsAssignable(gitLabUserID int64) bool {
	for _, member := range s.Members {
		if member.GitLabUserID == gitLabUserID {
			return member.Assignable()
		}
	}
	return false
}

func (s Snapshot) IsMemberOf(gitLabUserID int64, teamKey string) bool {
	for _, member := range s.Members {
		if member.GitLabUserID != gitLabUserID {
			continue
		}
		for _, key := range member.TeamKeys {
			if key == teamKey {
				return true
			}
		}
	}
	return false
}

func (s Snapshot) Team(teamKey string) (Team, bool) {
	for _, team := range s.Teams {
		if team.Key == teamKey && team.Active {
			return team, true
		}
	}
	return Team{}, false
}

func validMemberState(state MemberState) bool {
	switch state {
	case MemberActive, MemberBlocked, MemberDeactivated:
		return true
	default:
		return false
	}
}

func validMilestoneKind(kind MilestoneKind) bool {
	switch kind {
	case MilestoneOrganizing, MilestoneStandup, MilestoneConference:
		return true
	default:
		return false
	}
}
