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

var (
	ErrUnsupportedVersion  = errors.New("unsupported directory version")
	ErrInvalidTeam         = errors.New("invalid directory team")
	ErrDuplicateTeam       = errors.New("duplicate directory team")
	ErrDuplicateUsername   = errors.New("duplicate GitLab username")
	ErrSnapshotNotFound    = errors.New("directory snapshot not found")
	ErrPreferencesNotFound = errors.New("user preferences not found")
)

type File struct {
	Version int
	Teams   []TeamConfig
}

type TeamConfig struct {
	Key         string
	Name        string
	TitlePrefix string
	GitLabLabel string
	Active      bool
	Members     []string
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
	DirectoryMemberUsernames []string
}

type MissingMember struct {
	TeamKey  string
	Username string
}

type Snapshot struct {
	Teams          []Team
	Members        []Member
	SourceRevision string
	SyncedAt       time.Time
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
		teams = append(teams, team)
	}

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

	return Snapshot{Teams: teams, Members: members, SourceRevision: sourceRevision, SyncedAt: syncedAt.UTC()}, missing, nil
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
