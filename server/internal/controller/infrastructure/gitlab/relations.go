package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apprelation "example.com/project-template/internal/controller/application/cardrelation"
	"example.com/project-template/internal/domain/identity"
)

const relationshipWorkItemFields = `
  id iid title state webUrl
  workItemType { name }
  namespace { fullPath }
  widgets {
    type
    ... on WorkItemWidgetStatus { status { name category color } }
    ... on WorkItemWidgetLabels { labels { nodes { title color textColor } } }
    ... on WorkItemWidgetAssignees { assignees { nodes { id username name avatarUrl webUrl } } }
    ... on WorkItemWidgetStartAndDueDate { startDate dueDate }
    ... on WorkItemWidgetHierarchy { parent { id } }
  }`

const childItemsQuery = `query ChildItems($id: WorkItemID!, $first: Int!, $after: String) {
  workItem(id: $id) {
    widgets {
      ... on WorkItemWidgetHierarchy {
        children(first: $first, after: $after) {
          count
          pageInfo { hasNextPage endCursor }
          nodes {` + relationshipWorkItemFields + `}
        }
      }
    }
  }
}`

const linkedItemsQuery = `query LinkedItems($id: WorkItemID!, $first: Int!, $after: String) {
  workItem(id: $id) {
    widgets {
      ... on WorkItemWidgetLinkedItems {
        linkedItems(first: $first, after: $after) {
          pageInfo { hasNextPage endCursor }
          nodes { linkType workItem {` + relationshipWorkItemFields + `} }
        }
      }
    }
  }
}`

const childCandidatesQuery = `query ChildCandidates($fullPath: ID!, $sourceId: WorkItemID!, $search: String, $iids: [String!]) {
  project(fullPath: $fullPath) {
    workItems(first: 20, search: $search, iids: $iids, types: [TASK], sort: UPDATED_DESC) {
      nodes {` + relationshipWorkItemFields + `}
    }
  }
  workItem(id: $sourceId) { id }
}`

const linkedCandidatesQuery = `query LinkedCandidates($fullPath: ID!, $sourceId: WorkItemID!, $search: String, $iids: [String!]) {
  project(fullPath: $fullPath) {
    workItems(first: 20, search: $search, iids: $iids, types: [ISSUE, TASK], sort: UPDATED_DESC) {
      nodes {` + relationshipWorkItemFields + `}
    }
  }
  workItem(id: $sourceId) {
    widgets {
      ... on WorkItemWidgetLinkedItems {
        linkedItems(first: 100) { nodes { workItem { id } } }
      }
    }
  }
}`

const relationshipTargetQuery = `query RelationshipTarget($id: WorkItemID!) {
  workItem(id: $id) {` + relationshipWorkItemFields + `}
}`

const workItemTypesQuery = `query RelationshipWorkItemTypes($fullPath: ID!) {
  project(fullPath: $fullPath) { workItemTypes { nodes { id name } } }
}`

const createChildMutation = `mutation CreateChild($input: WorkItemCreateInput!) {
  workItemCreate(input: $input) { errors workItem {` + relationshipWorkItemFields + `} }
}`

const attachChildMutation = `mutation AttachChild($input: WorkItemHierarchyAddChildrenItemsInput!) {
  workItemHierarchyAddChildrenItems(input: $input) { errors addedChildren { id } }
}`

const detachChildMutation = `mutation DetachChild($input: WorkItemUpdateInput!) {
  workItemUpdate(input: $input) { errors }
}`

const addLinkedItemMutation = `mutation AddLinkedItem($input: WorkItemAddLinkedItemsInput!) {
  workItemAddLinkedItems(input: $input) { errors message }
}`

const removeLinkedItemMutation = `mutation RemoveLinkedItem($input: WorkItemRemoveLinkedItemsInput!) {
  workItemRemoveLinkedItems(input: $input) { errors message }
}`

type relationshipWorkItemNode struct {
	ID           string `json:"id"`
	IID          string `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	WebURL       string `json:"webUrl"`
	WorkItemType struct {
		Name string `json:"name"`
	} `json:"workItemType"`
	Namespace struct {
		FullPath string `json:"fullPath"`
	} `json:"namespace"`
	Widgets []relationshipWidget `json:"widgets"`
}

type relationshipWidget struct {
	Type      string `json:"type"`
	StartDate string `json:"startDate"`
	DueDate   string `json:"dueDate"`
	Status    *struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Color    string `json:"color"`
	} `json:"status"`
	Labels struct {
		Nodes []struct {
			Title     string `json:"title"`
			Color     string `json:"color"`
			TextColor string `json:"textColor"`
		} `json:"nodes"`
	} `json:"labels"`
	Assignees struct {
		Nodes []struct {
			ID        string `json:"id"`
			Username  string `json:"username"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatarUrl"`
			WebURL    string `json:"webUrl"`
		} `json:"nodes"`
	} `json:"assignees"`
	Parent *struct {
		ID string `json:"id"`
	} `json:"parent"`
}

type relationshipPageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

type childItemsConnection struct {
	Count    int32                      `json:"count"`
	PageInfo relationshipPageInfo       `json:"pageInfo"`
	Nodes    []relationshipWorkItemNode `json:"nodes"`
}

type linkedItemsConnection struct {
	PageInfo relationshipPageInfo `json:"pageInfo"`
	Nodes    []struct {
		LinkType string                   `json:"linkType"`
		WorkItem relationshipWorkItemNode `json:"workItem"`
	} `json:"nodes"`
}

func (c *Client) ChildItems(
	ctx context.Context,
	parentID int64,
	query apprelation.PageQuery,
	actorAccessToken string,
) (apprelation.ChildPage, error) {
	var data struct {
		WorkItem *struct {
			Widgets []struct {
				Children childItemsConnection `json:"children"`
			} `json:"widgets"`
		} `json:"workItem"`
	}
	err := c.graphQL(
		ctx,
		childItemsQuery,
		map[string]any{"id": workItemGlobalID(parentID), "first": query.Limit, "after": nullableCursor(query.Cursor)},
		"",
		actorAccessToken,
		&data,
	)
	if err != nil {
		return apprelation.ChildPage{}, mapRelationshipError(err)
	}
	if data.WorkItem == nil {
		return apprelation.ChildPage{}, apprelation.ErrWorkItemNotFound
	}
	connection := childItemsConnection{Nodes: []relationshipWorkItemNode{}}
	for _, widget := range data.WorkItem.Widgets {
		if len(widget.Children.Nodes) > 0 || widget.Children.Count > 0 {
			connection = widget.Children
			break
		}
	}
	items := make([]apprelation.WorkItem, 0, len(connection.Nodes))
	for _, node := range connection.Nodes {
		items = append(items, mapRelationshipWorkItem(node))
	}
	return apprelation.ChildPage{
		Items: items, TotalCount: connection.Count,
		NextCursor: nextCursor(connection.PageInfo),
	}, nil
}

func (c *Client) LinkedItems(
	ctx context.Context,
	workItemID int64,
	query apprelation.PageQuery,
	actorAccessToken string,
) (apprelation.LinkedPage, error) {
	var data struct {
		WorkItem *struct {
			Widgets []struct {
				LinkedItems linkedItemsConnection `json:"linkedItems"`
			} `json:"widgets"`
		} `json:"workItem"`
	}
	err := c.graphQL(
		ctx,
		linkedItemsQuery,
		map[string]any{"id": workItemGlobalID(workItemID), "first": query.Limit, "after": nullableCursor(query.Cursor)},
		"",
		actorAccessToken,
		&data,
	)
	if err != nil {
		return apprelation.LinkedPage{}, mapRelationshipError(err)
	}
	if data.WorkItem == nil {
		return apprelation.LinkedPage{}, apprelation.ErrWorkItemNotFound
	}
	connection := linkedItemsConnection{}
	for _, widget := range data.WorkItem.Widgets {
		if len(widget.LinkedItems.Nodes) > 0 {
			connection = widget.LinkedItems
			break
		}
	}
	items := make([]apprelation.LinkedItem, 0, len(connection.Nodes))
	for _, node := range connection.Nodes {
		items = append(items, apprelation.LinkedItem{
			WorkItem: mapRelationshipWorkItem(node.WorkItem),
			LinkType: apprelation.LinkType(strings.ToLower(node.LinkType)),
		})
	}
	return apprelation.LinkedPage{
		// Unlike most GitLab GraphQL connections, LinkedWorkItemTypeConnection
		// does not expose count. The API currently requests one page for the
		// drawer, so report the number of relationships in that page.
		Items: items, TotalCount: int32(len(connection.Nodes)),
		NextCursor: nextCursor(connection.PageInfo),
	}, nil
}

func (c *Client) RelationshipCandidates(
	ctx context.Context,
	sourceID int64,
	kind apprelation.RelationshipKind,
	query apprelation.CandidateQuery,
	actorAccessToken string,
) ([]apprelation.WorkItem, error) {
	variables := map[string]any{
		"fullPath": c.config.ProjectPath,
		"sourceId": workItemGlobalID(sourceID),
		"search":   nil,
		"iids":     nil,
	}
	if query.IID != nil {
		variables["iids"] = []string{strconv.FormatInt(*query.IID, 10)}
	} else {
		variables["search"] = query.Text
	}
	var data struct {
		Project struct {
			WorkItems struct {
				Nodes []relationshipWorkItemNode `json:"nodes"`
			} `json:"workItems"`
		} `json:"project"`
		WorkItem *struct {
			ID      string `json:"id"`
			Widgets []struct {
				LinkedItems struct {
					Nodes []struct {
						WorkItem struct {
							ID string `json:"id"`
						} `json:"workItem"`
					} `json:"nodes"`
				} `json:"linkedItems"`
			} `json:"widgets"`
		} `json:"workItem"`
	}
	graphQuery := childCandidatesQuery
	if kind == apprelation.RelationshipKindLinked {
		graphQuery = linkedCandidatesQuery
	}
	if err := c.graphQL(ctx, graphQuery, variables, "", actorAccessToken, &data); err != nil {
		return nil, mapRelationshipError(err)
	}
	if data.WorkItem == nil {
		return nil, apprelation.ErrWorkItemNotFound
	}
	excluded := map[int64]struct{}{sourceID: {}}
	if kind == apprelation.RelationshipKindLinked {
		for _, widget := range data.WorkItem.Widgets {
			for _, linked := range widget.LinkedItems.Nodes {
				excluded[parseGlobalID(linked.WorkItem.ID)] = struct{}{}
			}
		}
	}
	items := make([]apprelation.WorkItem, 0, len(data.Project.WorkItems.Nodes))
	for _, node := range data.Project.WorkItems.Nodes {
		id := parseGlobalID(node.ID)
		if _, skip := excluded[id]; skip || node.Namespace.FullPath != c.config.ProjectPath {
			continue
		}
		if kind == apprelation.RelationshipKindChild && currentParentID(node) != 0 {
			continue
		}
		items = append(items, mapRelationshipWorkItem(node))
	}
	return items, nil
}

func (c *Client) CreateChild(
	ctx context.Context,
	parentID int64,
	title string,
	actorAccessToken string,
) (apprelation.WorkItem, error) {
	taskTypeID, err := c.relationshipWorkItemTypeID(ctx, "Task", actorAccessToken)
	if err != nil {
		return apprelation.WorkItem{}, err
	}
	input := map[string]any{
		"namespacePath":  c.config.ProjectPath,
		"title":          title,
		"workItemTypeId": taskTypeID,
		"hierarchyWidget": map[string]any{
			"parentId": workItemGlobalID(parentID),
		},
	}
	var data struct {
		WorkItemCreate struct {
			Errors   []string                 `json:"errors"`
			WorkItem relationshipWorkItemNode `json:"workItem"`
		} `json:"workItemCreate"`
	}
	if err := c.graphQL(ctx, createChildMutation, map[string]any{"input": input}, "", actorAccessToken, &data); err != nil {
		return apprelation.WorkItem{}, mapRelationshipError(err)
	}
	if err := relationshipMutationError(data.WorkItemCreate.Errors); err != nil {
		return apprelation.WorkItem{}, err
	}
	if parseGlobalID(data.WorkItemCreate.WorkItem.ID) == 0 {
		return apprelation.WorkItem{}, apprelation.ErrWorkItemNotFound
	}
	return mapRelationshipWorkItem(data.WorkItemCreate.WorkItem), nil
}

func (c *Client) AttachChild(ctx context.Context, parentID, childID int64, actorAccessToken string) error {
	child, err := c.relationshipTarget(ctx, childID, actorAccessToken)
	if err != nil {
		return err
	}
	if err := c.validateRelationshipTarget(child, apprelation.RelationshipKindChild); err != nil {
		return err
	}
	if currentParentID(child) != 0 {
		return apprelation.ErrRelationConflict
	}
	var data struct {
		WorkItemHierarchyAddChildrenItems struct {
			Errors []string `json:"errors"`
		} `json:"workItemHierarchyAddChildrenItems"`
	}
	input := map[string]any{"id": workItemGlobalID(parentID), "childrenIds": []string{workItemGlobalID(childID)}}
	if err := c.graphQL(ctx, attachChildMutation, map[string]any{"input": input}, "", actorAccessToken, &data); err != nil {
		return mapRelationshipError(err)
	}
	return relationshipMutationError(data.WorkItemHierarchyAddChildrenItems.Errors)
}

func (c *Client) DetachChild(ctx context.Context, parentID, childID int64, actorAccessToken string) error {
	child, err := c.relationshipTarget(ctx, childID, actorAccessToken)
	if err != nil {
		return err
	}
	if err := c.validateRelationshipTarget(child, apprelation.RelationshipKindChild); err != nil {
		return err
	}
	if currentParentID(child) != parentID {
		return apprelation.ErrRelationConflict
	}
	var data struct {
		WorkItemUpdate struct {
			Errors []string `json:"errors"`
		} `json:"workItemUpdate"`
	}
	input := map[string]any{"id": workItemGlobalID(childID), "hierarchyWidget": map[string]any{"parentId": nil}}
	if err := c.graphQL(ctx, detachChildMutation, map[string]any{"input": input}, "", actorAccessToken, &data); err != nil {
		return mapRelationshipError(err)
	}
	return relationshipMutationError(data.WorkItemUpdate.Errors)
}

func (c *Client) AddLinks(
	ctx context.Context,
	sourceID int64,
	targetIDs []int64,
	linkType apprelation.LinkType,
	actorAccessToken string,
) error {
	workItemIDs := make([]string, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		target, err := c.relationshipTarget(ctx, targetID, actorAccessToken)
		if err != nil {
			return err
		}
		if err := c.validateRelationshipTarget(target, apprelation.RelationshipKindLinked); err != nil {
			return err
		}
		workItemIDs = append(workItemIDs, workItemGlobalID(targetID))
	}
	var data struct {
		WorkItemAddLinkedItems struct {
			Errors []string `json:"errors"`
		} `json:"workItemAddLinkedItems"`
	}
	input := map[string]any{
		"id": workItemGlobalID(sourceID), "workItemsIds": workItemIDs,
		"linkType": graphQLLinkType(linkType),
	}
	if err := c.graphQL(ctx, addLinkedItemMutation, map[string]any{"input": input}, "", actorAccessToken, &data); err != nil {
		return mapRelationshipError(err)
	}
	return relationshipMutationError(data.WorkItemAddLinkedItems.Errors)
}

func (c *Client) RemoveLink(ctx context.Context, sourceID, targetID int64, actorAccessToken string) error {
	target, err := c.relationshipTarget(ctx, targetID, actorAccessToken)
	if err != nil {
		return err
	}
	if err := c.validateRelationshipTarget(target, apprelation.RelationshipKindLinked); err != nil {
		return err
	}
	var data struct {
		WorkItemRemoveLinkedItems struct {
			Errors []string `json:"errors"`
		} `json:"workItemRemoveLinkedItems"`
	}
	input := map[string]any{"id": workItemGlobalID(sourceID), "workItemsIds": []string{workItemGlobalID(targetID)}}
	if err := c.graphQL(ctx, removeLinkedItemMutation, map[string]any{"input": input}, "", actorAccessToken, &data); err != nil {
		return mapRelationshipError(err)
	}
	return relationshipMutationError(data.WorkItemRemoveLinkedItems.Errors)
}

func (c *Client) relationshipTarget(
	ctx context.Context,
	workItemID int64,
	actorAccessToken string,
) (relationshipWorkItemNode, error) {
	var data struct {
		WorkItem *relationshipWorkItemNode `json:"workItem"`
	}
	if err := c.graphQL(
		ctx,
		relationshipTargetQuery,
		map[string]any{"id": workItemGlobalID(workItemID)},
		"",
		actorAccessToken,
		&data,
	); err != nil {
		return relationshipWorkItemNode{}, mapRelationshipError(err)
	}
	if data.WorkItem == nil {
		return relationshipWorkItemNode{}, apprelation.ErrWorkItemNotFound
	}
	return *data.WorkItem, nil
}

func (c *Client) relationshipWorkItemTypeID(ctx context.Context, name, actorAccessToken string) (string, error) {
	var data struct {
		Project struct {
			WorkItemTypes struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"workItemTypes"`
		} `json:"project"`
	}
	if err := c.graphQL(
		ctx,
		workItemTypesQuery,
		map[string]any{"fullPath": c.config.ProjectPath},
		"",
		actorAccessToken,
		&data,
	); err != nil {
		return "", mapRelationshipError(err)
	}
	for _, itemType := range data.Project.WorkItemTypes.Nodes {
		if strings.EqualFold(itemType.Name, name) {
			return itemType.ID, nil
		}
	}
	return "", apprelation.ErrFeatureUnavailable
}

func (c *Client) validateRelationshipTarget(node relationshipWorkItemNode, kind apprelation.RelationshipKind) error {
	if node.Namespace.FullPath != c.config.ProjectPath {
		return apprelation.ErrInvalidRelation
	}
	itemType := strings.ToLower(node.WorkItemType.Name)
	if kind == apprelation.RelationshipKindChild && itemType != string(apprelation.WorkItemTypeTask) {
		return apprelation.ErrInvalidRelation
	}
	if kind == apprelation.RelationshipKindLinked && itemType != string(apprelation.WorkItemTypeIssue) && itemType != string(apprelation.WorkItemTypeTask) {
		return apprelation.ErrInvalidRelation
	}
	return nil
}

func mapRelationshipWorkItem(node relationshipWorkItemNode) apprelation.WorkItem {
	iid, _ := strconv.ParseInt(node.IID, 10, 64)
	item := apprelation.WorkItem{
		GitLabWorkItemID: parseGlobalID(node.ID), IID: iid,
		Type:  apprelation.WorkItemType(strings.ToLower(node.WorkItemType.Name)),
		Title: node.Title, State: apprelation.WorkItemState(strings.ToLower(node.State)), WebURL: node.WebURL,
		Assignees: []apprelation.Assignee{}, Labels: []apprelation.Label{},
	}
	for _, widget := range node.Widgets {
		switch widget.Type {
		case "STATUS":
			if widget.Status != nil {
				item.Status = &apprelation.Status{
					Name: widget.Status.Name, Category: widget.Status.Category, Color: widget.Status.Color,
				}
			}
		case "LABELS":
			for _, label := range widget.Labels.Nodes {
				item.Labels = append(item.Labels, apprelation.Label{
					Name: label.Title, Color: label.Color, TextColor: label.TextColor,
				})
			}
		case "ASSIGNEES":
			for _, assignee := range widget.Assignees.Nodes {
				item.Assignees = append(item.Assignees, apprelation.Assignee{
					GitLabUserID: parseGlobalID(assignee.ID), Username: assignee.Username,
					DisplayName: assignee.Name, AvatarURL: assignee.AvatarURL, ProfileURL: assignee.WebURL,
				})
			}
		case "START_AND_DUE_DATE":
			item.StartDate, item.DueDate = widget.StartDate, widget.DueDate
		}
	}
	return item
}

func currentParentID(node relationshipWorkItemNode) int64 {
	for _, widget := range node.Widgets {
		if widget.Type == "HIERARCHY" && widget.Parent != nil {
			return parseGlobalID(widget.Parent.ID)
		}
	}
	return 0
}

func relationshipMutationError(messages []string) error {
	if len(messages) == 0 {
		return nil
	}
	return mapRelationshipError(fmt.Errorf("GitLab work item relationship: %s", strings.Join(messages, "; ")))
}

func mapRelationshipError(err error) error {
	if err == nil || errors.Is(err, identity.ErrGitLabUnavailable) {
		return err
	}
	var statusError *httpStatusError
	if errors.As(err, &statusError) {
		switch statusError.status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return apprelation.ErrGitLabForbidden
		case http.StatusNotFound:
			return apprelation.ErrWorkItemNotFound
		}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "licensed feature"),
		strings.Contains(message, "blocked work items are not available"),
		strings.Contains(message, "blocking relationships are not available"):
		return apprelation.ErrFeatureUnavailable
	case strings.Contains(message, "already linked"),
		strings.Contains(message, "already has a parent"),
		strings.Contains(message, "already assigned"),
		strings.Contains(message, "not a child of"):
		return apprelation.ErrRelationConflict
	case strings.Contains(message, "not found"):
		return apprelation.ErrWorkItemNotFound
	case strings.Contains(message, "permission"), strings.Contains(message, "forbidden"):
		return apprelation.ErrGitLabForbidden
	case strings.Contains(message, "cannot be added"),
		strings.Contains(message, "must be in the same project"),
		strings.Contains(message, "invalid link type"),
		strings.Contains(message, "cannot link"):
		return apprelation.ErrInvalidRelation
	default:
		return err
	}
}

func graphQLLinkType(linkType apprelation.LinkType) string {
	switch linkType {
	case apprelation.LinkTypeBlocks:
		return "BLOCKS"
	case apprelation.LinkTypeIsBlockedBy:
		return "BLOCKED_BY"
	case apprelation.LinkTypeRelatesTo:
		return "RELATED"
	default:
		return ""
	}
}

func workItemGlobalID(id int64) string {
	return fmt.Sprintf("gid://gitlab/WorkItem/%d", id)
}

func nullableCursor(cursor string) any {
	if cursor == "" {
		return nil
	}
	return cursor
}

func nextCursor(pageInfo relationshipPageInfo) string {
	if !pageInfo.HasNextPage || pageInfo.EndCursor == nil {
		return ""
	}
	return *pageInfo.EndCursor
}
