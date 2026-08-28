package cardrelation

type WorkItemType string
type WorkItemState string
type LinkType string
type RelationshipKind string

const (
	WorkItemTypeIssue WorkItemType = "issue"
	WorkItemTypeTask  WorkItemType = "task"

	WorkItemStateOpen   WorkItemState = "open"
	WorkItemStateClosed WorkItemState = "closed"

	LinkTypeRelatesTo   LinkType = "relates_to"
	LinkTypeBlocks      LinkType = "blocks"
	LinkTypeIsBlockedBy LinkType = "is_blocked_by"

	RelationshipKindChild  RelationshipKind = "child"
	RelationshipKindLinked RelationshipKind = "linked"
)

type Assignee struct {
	GitLabUserID int64
	Username     string
	DisplayName  string
	AvatarURL    string
	ProfileURL   string
}

type Status struct {
	Name     string
	Category string
	Color    string
}

type Label struct {
	Name      string
	Color     string
	TextColor string
}

type WorkItem struct {
	GitLabWorkItemID int64
	IID              int64
	Type             WorkItemType
	Title            string
	State            WorkItemState
	WebURL           string
	Status           *Status
	Assignees        []Assignee
	StartDate        string
	DueDate          string
	Labels           []Label
}

type LinkedItem struct {
	WorkItem
	LinkType LinkType
}

type PageQuery struct {
	Cursor string
	Limit  int32
}

type ChildPage struct {
	Items      []WorkItem
	TotalCount int32
	NextCursor string
}

type LinkedPage struct {
	Items      []LinkedItem
	TotalCount int32
	NextCursor string
}

type CandidateQuery struct {
	Text string
	IID  *int64
}

type CreateChildInput struct {
	ActorUserID string
	IssueIID    int64
	Title       string
}

type ChildRelationInput struct {
	ActorUserID string
	IssueIID    int64
	WorkItemID  int64
}

type LinkInput struct {
	ActorUserID string
	IssueIID    int64
	WorkItemIDs []int64
	LinkType    LinkType
}

type SearchInput struct {
	ActorUserID string
	IssueIID    int64
	Kind        RelationshipKind
	Query       string
}
