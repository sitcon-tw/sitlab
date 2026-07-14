package httpserver

import (
	"time"

	appboard "example.com/project-template/internal/controller/application/board"
	appbootstrap "example.com/project-template/internal/controller/application/bootstrap"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type userResponse struct {
	ID           string  `json:"id"`
	GitLabUserID int64   `json:"gitLabUserId"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarURL    *string `json:"avatarUrl"`
	ProfileURL   string  `json:"profileUrl"`
	AccessLevel  int32   `json:"accessLevel"`
}

type teamResponse struct {
	Key                 string  `json:"key"`
	Name                string  `json:"name"`
	TitlePrefix         string  `json:"titlePrefix"`
	GitLabLabel         string  `json:"gitLabLabel"`
	Active              bool    `json:"active"`
	SortOrder           int32   `json:"sortOrder"`
	MemberGitLabUserIDs []int64 `json:"memberGitLabUserIds"`
}

type directoryMemberResponse struct {
	GitLabUserID int64                 `json:"gitLabUserId"`
	Username     string                `json:"username"`
	DisplayName  string                `json:"displayName"`
	AvatarURL    *string               `json:"avatarUrl"`
	ProfileURL   string                `json:"profileUrl"`
	AccessLevel  int32                 `json:"accessLevel"`
	State        directory.MemberState `json:"state"`
	TeamKeys     []string              `json:"teamKeys"`
}

type preferencesResponse struct {
	DefaultTeamKey    *string    `json:"defaultTeamKey"`
	ConfirmedAt       *time.Time `json:"confirmedAt"`
	DirectoryTeamKeys []string   `json:"directoryTeamKeys"`
}

type boardListResponse struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	GitLabLabel string `json:"gitLabLabel"`
	Position    int32  `json:"position"`
	Closed      bool   `json:"closed"`
	Color       string `json:"color"`
}

type cardResponse struct {
	IssueIID             int64                `json:"issueIid"`
	IssueID              *int64               `json:"issueId"`
	Title                string               `json:"title"`
	WebURL               *string              `json:"webUrl"`
	ListKey              string               `json:"listKey"`
	Position             int32                `json:"position"`
	TeamKey              string               `json:"teamKey"`
	AssigneeGitLabUserID *int64               `json:"assigneeGitLabUserId"`
	DueDate              *string              `json:"dueDate"`
	Labels               []string             `json:"labels"`
	SyncState            board.OperationState `json:"syncState"`
	SyncError            *string              `json:"syncError"`
	PendingOperationID   *string              `json:"pendingOperationId"`
	UpdatedAt            time.Time            `json:"updatedAt"`
}

type operationResponse struct {
	ID        string               `json:"id"`
	Kind      board.OperationKind  `json:"kind"`
	State     board.OperationState `json:"state"`
	Attempts  int32                `json:"attempts"`
	LastError *string              `json:"lastError"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
}

type boardSnapshotResponse struct {
	Lists    []boardListResponse `json:"lists"`
	Cards    []cardResponse      `json:"cards"`
	SyncedAt time.Time           `json:"syncedAt"`
}

type syncStatusResponse struct {
	State         string    `json:"state"`
	LastSuccessAt time.Time `json:"lastSuccessAt"`
	Message       *string   `json:"message"`
}

type bootstrapResponse struct {
	Me          userResponse              `json:"me"`
	CSRFToken   string                    `json:"csrfToken"`
	Teams       []teamResponse            `json:"teams"`
	Members     []directoryMemberResponse `json:"members"`
	Board       boardSnapshotResponse     `json:"board"`
	Preferences preferencesResponse       `json:"preferences"`
	Sync        syncStatusResponse        `json:"sync"`
}

func mapUser(item identity.User) userResponse {
	return userResponse{
		ID: item.ID, GitLabUserID: item.GitLabUserID, Username: item.Username,
		DisplayName: item.DisplayName, AvatarURL: optionalString(item.AvatarURL),
		ProfileURL: item.ProfileURL, AccessLevel: item.AccessLevel,
	}
}

func mapTeam(item directory.Team) teamResponse {
	ids := append([]int64{}, item.MemberGitLabUserIDs...)
	return teamResponse{
		Key: item.Key, Name: item.Name, TitlePrefix: item.TitlePrefix,
		GitLabLabel: item.GitLabLabel, Active: item.Active,
		SortOrder: item.SortOrder, MemberGitLabUserIDs: ids,
	}
}

func mapDirectoryMember(item directory.Member) directoryMemberResponse {
	return directoryMemberResponse{
		GitLabUserID: item.GitLabUserID, Username: item.Username,
		DisplayName: item.DisplayName, AvatarURL: optionalString(item.AvatarURL),
		ProfileURL: item.ProfileURL, AccessLevel: item.AccessLevel,
		State: item.State, TeamKeys: append([]string{}, item.TeamKeys...),
	}
}

func mapPreferences(item appdirectory.Preferences) preferencesResponse {
	return preferencesResponse{
		DefaultTeamKey: item.DefaultTeamKey, ConfirmedAt: item.ConfirmedAt,
		DirectoryTeamKeys: append([]string{}, item.DirectoryTeamKeys...),
	}
}

func mapBoardList(item board.List) boardListResponse {
	return boardListResponse{
		Key: item.Key, Name: item.Name, GitLabLabel: item.GitLabLabel,
		Position: item.Position, Closed: item.Closed, Color: item.Color,
	}
}

func mapCard(item board.Card) cardResponse {
	return cardResponse{
		IssueIID: item.IssueIID, IssueID: item.GitLabIssueID,
		Title: item.Title, WebURL: optionalString(item.WebURL), ListKey: item.ListKey,
		Position: item.Position, TeamKey: item.TeamKey,
		AssigneeGitLabUserID: item.AssigneeGitLabUserID,
		DueDate:              optionalString(item.DueDate), Labels: append([]string{}, item.Labels...),
		SyncState: item.SyncState, SyncError: optionalString(item.SyncError),
		PendingOperationID: optionalString(item.PendingOperationID), UpdatedAt: item.UpdatedAt,
	}
}

func mapOperation(item board.Operation) operationResponse {
	return operationResponse{
		ID: item.ID, Kind: item.Kind, State: item.State, Attempts: item.Attempts,
		LastError: optionalString(item.LastError), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func mapBoardSnapshot(item appboard.Snapshot) boardSnapshotResponse {
	lists := make([]boardListResponse, 0, len(item.Lists))
	for _, list := range item.Lists {
		lists = append(lists, mapBoardList(list))
	}
	cards := make([]cardResponse, 0, len(item.Cards))
	for _, card := range item.Cards {
		cards = append(cards, mapCard(card))
	}
	return boardSnapshotResponse{Lists: lists, Cards: cards, SyncedAt: item.SyncedAt}
}

func mapBootstrap(item appbootstrap.Result) bootstrapResponse {
	teams := make([]teamResponse, 0, len(item.Directory.Teams))
	for _, team := range item.Directory.Teams {
		teams = append(teams, mapTeam(team))
	}
	members := make([]directoryMemberResponse, 0, len(item.Directory.Members))
	for _, member := range item.Directory.Members {
		members = append(members, mapDirectoryMember(member))
	}
	return bootstrapResponse{
		Me: mapUser(item.Me), CSRFToken: item.CSRFToken, Teams: teams, Members: members,
		Board: mapBoardSnapshot(item.Board), Preferences: mapPreferences(item.Preferences),
		Sync: syncStatusResponse{
			State: item.Sync.State, LastSuccessAt: item.Sync.LastSuccessAt,
			Message: optionalString(item.Sync.Message),
		},
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
