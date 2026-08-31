package httpserver

import (
	"time"

	appboard "example.com/project-template/internal/controller/application/board"
	appbootstrap "example.com/project-template/internal/controller/application/bootstrap"
	appactivity "example.com/project-template/internal/controller/application/cardactivity"
	apprelation "example.com/project-template/internal/controller/application/cardrelation"
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
	LeaderGitLabUserIDs []int64 `json:"leaderGitLabUserIds"`
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
	Key      string `json:"key"`
	Name     string `json:"name"`
	Position int32  `json:"position"`
	Closed   bool   `json:"closed"`
	Color    string `json:"color"`
}

type cardResponse struct {
	IssueIID              int64                `json:"issueIid"`
	IssueID               *int64               `json:"issueId"`
	Title                 string               `json:"title"`
	Description           string               `json:"description"`
	WebURL                *string              `json:"webUrl"`
	ListKey               string               `json:"listKey"`
	Position              int32                `json:"position"`
	TeamKey               string               `json:"teamKey"`
	AssigneeGitLabUserIDs []int64              `json:"assigneeGitLabUserIds"`
	StartDate             *string              `json:"startDate"`
	DueDate               *string              `json:"dueDate"`
	Labels                []string             `json:"labels"`
	GitLabStatusName      *string              `json:"gitLabStatusName"`
	SyncState             board.OperationState `json:"syncState"`
	SyncError             *string              `json:"syncError"`
	PendingOperationID    *string              `json:"pendingOperationId"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
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

type projectLabelResponse struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	TextColor   string  `json:"textColor"`
	Description *string `json:"description"`
}

type commentAuthorResponse struct {
	GitLabUserID int64   `json:"gitLabUserId"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarURL    *string `json:"avatarUrl"`
	ProfileURL   string  `json:"profileUrl"`
}

type cardCommentResponse struct {
	ID        int64                 `json:"id"`
	Body      string                `json:"body"`
	Author    commentAuthorResponse `json:"author"`
	System    bool                  `json:"system"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

type workItemAssigneeResponse struct {
	GitLabUserID int64   `json:"gitLabUserId"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarURL    *string `json:"avatarUrl"`
	ProfileURL   string  `json:"profileUrl"`
}

type workItemStatusResponse struct {
	Name     string  `json:"name"`
	Category *string `json:"category"`
	Color    *string `json:"color"`
}

type workItemLabelResponse struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	TextColor string `json:"textColor"`
}

type workItemResponse struct {
	GitLabWorkItemID int64                      `json:"gitLabWorkItemId"`
	IID              int64                      `json:"iid"`
	Type             apprelation.WorkItemType   `json:"type"`
	Title            string                     `json:"title"`
	State            apprelation.WorkItemState  `json:"state"`
	WebURL           string                     `json:"webUrl"`
	Status           *workItemStatusResponse    `json:"status"`
	Assignees        []workItemAssigneeResponse `json:"assignees"`
	StartDate        *string                    `json:"startDate"`
	DueDate          *string                    `json:"dueDate"`
	Labels           []workItemLabelResponse    `json:"labels"`
}

type linkedWorkItemResponse struct {
	workItemResponse
	LinkType apprelation.LinkType `json:"linkType"`
}

type directoryMilestoneResponse struct {
	Name string `json:"name"`
	Date string `json:"date"`
	Kind string `json:"kind"`
}

type bootstrapResponse struct {
	Revision    string                       `json:"revision"`
	Me          userResponse                 `json:"me"`
	CSRFToken   string                       `json:"csrfToken"`
	Teams       []teamResponse               `json:"teams"`
	Members     []directoryMemberResponse    `json:"members"`
	Milestones  []directoryMilestoneResponse `json:"milestones"`
	Board       boardSnapshotResponse        `json:"board"`
	Preferences preferencesResponse          `json:"preferences"`
	Sync        syncStatusResponse           `json:"sync"`
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
	leaderIDs := append([]int64{}, item.LeaderGitLabUserIDs...)
	return teamResponse{
		Key: item.Key, Name: item.Name, TitlePrefix: item.TitlePrefix,
		GitLabLabel: item.GitLabLabel, Active: item.Active,
		SortOrder: item.SortOrder, MemberGitLabUserIDs: ids, LeaderGitLabUserIDs: leaderIDs,
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

func mapDirectoryMilestone(item directory.Milestone) directoryMilestoneResponse {
	return directoryMilestoneResponse{Name: item.Name, Date: item.Date, Kind: string(item.Kind)}
}

func mapDirectoryMilestones(items []directory.Milestone) []directoryMilestoneResponse {
	milestones := make([]directoryMilestoneResponse, 0, len(items))
	for _, item := range items {
		milestones = append(milestones, mapDirectoryMilestone(item))
	}
	return milestones
}

func mapPreferences(item appdirectory.Preferences) preferencesResponse {
	return preferencesResponse{
		DefaultTeamKey: item.DefaultTeamKey, ConfirmedAt: item.ConfirmedAt,
		DirectoryTeamKeys: append([]string{}, item.DirectoryTeamKeys...),
	}
}

func mapBoardList(item board.List) boardListResponse {
	return boardListResponse{
		Key: item.Key, Name: item.Name,
		Position: item.Position, Closed: item.Closed, Color: item.Color,
	}
}

func mapCard(item board.Card) cardResponse {
	return cardResponse{
		IssueIID: item.IssueIID, IssueID: item.GitLabIssueID,
		Title: item.Title, Description: item.Description, WebURL: optionalString(item.WebURL), ListKey: item.ListKey,
		Position: item.Position, TeamKey: item.TeamKey,
		AssigneeGitLabUserIDs: append([]int64{}, item.AssigneeGitLabUserIDs...),
		StartDate:             optionalString(item.StartDate), DueDate: optionalString(item.DueDate), Labels: append([]string{}, item.Labels...),
		GitLabStatusName: optionalString(item.GitLabStatusName),
		SyncState:        item.SyncState, SyncError: optionalString(item.SyncError),
		PendingOperationID: optionalString(item.PendingOperationID), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func mapOperation(item board.Operation) operationResponse {
	return operationResponse{
		ID: item.ID, Kind: item.Kind, State: item.State, Attempts: item.Attempts,
		LastError: optionalString(item.LastError), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func mapProjectLabel(item appactivity.ProjectLabel) projectLabelResponse {
	return projectLabelResponse{
		ID: item.ID, Name: item.Name, Color: item.Color, TextColor: item.TextColor, Description: item.Description,
	}
}

func mapCardComment(item appactivity.Comment) cardCommentResponse {
	return cardCommentResponse{
		ID: item.ID, Body: item.Body, System: item.System, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		Author: commentAuthorResponse{
			GitLabUserID: item.Author.GitLabUserID, Username: item.Author.Username,
			DisplayName: item.Author.DisplayName, AvatarURL: optionalString(item.Author.AvatarURL), ProfileURL: item.Author.ProfileURL,
		},
	}
}

func mapWorkItem(item apprelation.WorkItem) workItemResponse {
	assignees := make([]workItemAssigneeResponse, 0, len(item.Assignees))
	for _, assignee := range item.Assignees {
		assignees = append(assignees, workItemAssigneeResponse{
			GitLabUserID: assignee.GitLabUserID, Username: assignee.Username,
			DisplayName: assignee.DisplayName, AvatarURL: optionalString(assignee.AvatarURL), ProfileURL: assignee.ProfileURL,
		})
	}
	labels := make([]workItemLabelResponse, 0, len(item.Labels))
	for _, label := range item.Labels {
		labels = append(labels, workItemLabelResponse{Name: label.Name, Color: label.Color, TextColor: label.TextColor})
	}
	var status *workItemStatusResponse
	if item.Status != nil {
		status = &workItemStatusResponse{
			Name: item.Status.Name, Category: optionalString(item.Status.Category), Color: optionalString(item.Status.Color),
		}
	}
	return workItemResponse{
		GitLabWorkItemID: item.GitLabWorkItemID, IID: item.IID, Type: item.Type,
		Title: item.Title, State: item.State, WebURL: item.WebURL, Status: status,
		Assignees: assignees, StartDate: optionalString(item.StartDate), DueDate: optionalString(item.DueDate), Labels: labels,
	}
}

func mapLinkedWorkItem(item apprelation.LinkedItem) linkedWorkItemResponse {
	return linkedWorkItemResponse{workItemResponse: mapWorkItem(item.WorkItem), LinkType: item.LinkType}
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
		Revision: item.Revision, Me: mapUser(item.Me), CSRFToken: item.CSRFToken, Teams: teams, Members: members,
		Milestones: mapDirectoryMilestones(item.Directory.Milestones),
		Board:      mapBoardSnapshot(item.Board), Preferences: mapPreferences(item.Preferences),
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
