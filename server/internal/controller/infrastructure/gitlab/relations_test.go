package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	apprelation "example.com/project-template/internal/controller/application/cardrelation"
)

func TestRelationshipReadsUseActorTokenAndMapMetadata(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer actor-token" || request.Header.Get("PRIVATE-TOKEN") != "" {
			t.Errorf("credentials = Authorization %q, PRIVATE-TOKEN %q", request.Header.Get("Authorization"), request.Header.Get("PRIVATE-TOKEN"))
		}
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.Contains(payload.Query, "query ChildItems"):
			return response(http.StatusOK, `{"data":{"workItem":{"widgets":[{"children":{"count":1,"pageInfo":{"hasNextPage":true,"endCursor":"next"},"nodes":[`+relationshipNodeJSON("9201", "201", "Task", "Add metrics", "")+`]}}]}}}`), nil
		case strings.Contains(payload.Query, "query LinkedItems"):
			if strings.Contains(payload.Query, "count") {
				t.Error("LinkedWorkItemTypeConnection does not expose count")
			}
			return response(http.StatusOK, `{"data":{"workItem":{"widgets":[{"linkedItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[{"linkType":"is_blocked_by","workItem":`+relationshipNodeJSON("9128", "128", "Issue", "Related issue", "")+`}]}}]}}}`), nil
		default:
			return response(http.StatusBadRequest, `{}`), nil
		}
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	children, err := client.ChildItems(context.Background(), 9001, apprelation.PageQuery{Limit: 50}, "actor-token")
	if err != nil || children.TotalCount != 1 || children.NextCursor != "next" || children.Items[0].Type != apprelation.WorkItemTypeTask {
		t.Fatalf("ChildItems() = %#v, %v", children, err)
	}
	item := children.Items[0]
	if item.Status == nil || item.Status.Name != "To do" || len(item.Assignees) != 1 || item.Assignees[0].DisplayName != "Alice" || len(item.Labels) != 1 {
		t.Fatalf("mapped child = %#v", item)
	}
	links, err := client.LinkedItems(context.Background(), 9001, apprelation.PageQuery{Limit: 50}, "actor-token")
	if err != nil || len(links.Items) != 1 || links.Items[0].LinkType != apprelation.LinkTypeIsBlockedBy || links.Items[0].IID != 128 {
		t.Fatalf("LinkedItems() = %#v, %v", links, err)
	}
}

func TestRelationshipCandidatesExcludeParentedTasksAndExistingLinks(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload.Query, "ChildCandidates") {
			return response(http.StatusOK, `{"data":{"project":{"workItems":{"nodes":[`+
				relationshipNodeJSON("9203", "203", "Task", "Available", "")+`,`+
				relationshipNodeJSON("9204", "204", "Task", "Already parented", "gid://gitlab/WorkItem/8999")+
				`]}},"workItem":{"id":"gid://gitlab/WorkItem/9001"}}}`), nil
		}
		return response(http.StatusOK, `{"data":{"project":{"workItems":{"nodes":[`+
			relationshipNodeJSON("9128", "128", "Issue", "Existing link", "")+`,`+
			relationshipNodeJSON("9129", "129", "Issue", "Available issue", "")+
			`]}},"workItem":{"widgets":[{"linkedItems":{"nodes":[{"workItem":{"id":"gid://gitlab/WorkItem/9128"}}]}}]}}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	children, err := client.RelationshipCandidates(
		context.Background(),
		9001,
		apprelation.RelationshipKindChild,
		apprelation.CandidateQuery{Text: "task"},
		"actor-token",
	)
	if err != nil || len(children) != 1 || children[0].IID != 203 {
		t.Fatalf("child candidates = %#v, %v", children, err)
	}
	links, err := client.RelationshipCandidates(
		context.Background(),
		9001,
		apprelation.RelationshipKindLinked,
		apprelation.CandidateQuery{Text: "issue"},
		"actor-token",
	)
	if err != nil || len(links) != 1 || links[0].IID != 129 {
		t.Fatalf("linked candidates = %#v, %v", links, err)
	}
}

func TestCreateAndAttachChildUseNativeHierarchyMutations(t *testing.T) {
	t.Parallel()
	var sawCreate, sawAttach bool
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.Contains(payload.Query, "RelationshipWorkItemTypes"):
			return response(http.StatusOK, `{"data":{"project":{"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/5","name":"Task"}]}}}}`), nil
		case strings.Contains(payload.Query, "mutation CreateChild"):
			sawCreate = true
			input := payload.Variables["input"].(map[string]any)
			if input["namespacePath"] != "sitcon-tw/2027" || input["workItemTypeId"] != "gid://gitlab/WorkItems::Type/5" || input["title"] != "Add metrics" {
				t.Errorf("create input = %#v", input)
			}
			hierarchy := input["hierarchyWidget"].(map[string]any)
			if hierarchy["parentId"] != "gid://gitlab/WorkItem/9001" {
				t.Errorf("hierarchy input = %#v", hierarchy)
			}
			return response(http.StatusOK, `{"data":{"workItemCreate":{"errors":[],"workItem":`+relationshipNodeJSON("9205", "205", "Task", "Add metrics", "gid://gitlab/WorkItem/9001")+`}}}`), nil
		case strings.Contains(payload.Query, "query RelationshipTarget"):
			return response(http.StatusOK, `{"data":{"workItem":`+relationshipNodeJSON("9203", "203", "Task", "Existing task", "")+`}}`), nil
		case strings.Contains(payload.Query, "mutation AttachChild"):
			sawAttach = true
			input := payload.Variables["input"].(map[string]any)
			if input["id"] != "gid://gitlab/WorkItem/9001" {
				t.Errorf("attach input = %#v", input)
			}
			return response(http.StatusOK, `{"data":{"workItemHierarchyAddChildrenItems":{"errors":[],"addedChildren":[{"id":"gid://gitlab/WorkItem/9203"}]}}}`), nil
		default:
			return response(http.StatusBadRequest, `{}`), nil
		}
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	created, err := client.CreateChild(context.Background(), 9001, "Add metrics", "actor-token")
	if err != nil || created.GitLabWorkItemID != 9205 {
		t.Fatalf("CreateChild() = %#v, %v", created, err)
	}
	if err := client.AttachChild(context.Background(), 9001, 9203, "actor-token"); err != nil {
		t.Fatalf("AttachChild() error = %v", err)
	}
	if !sawCreate || !sawAttach {
		t.Fatalf("mutations seen: create=%v attach=%v", sawCreate, sawAttach)
	}
}

func TestBlockingLicenseFailureHasStableError(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload.Query, "RelationshipTarget") {
			return response(http.StatusOK, `{"data":{"workItem":`+relationshipNodeJSON("9128", "128", "Issue", "Target", "")+`}}`), nil
		}
		return response(http.StatusOK, `{"data":{"workItemAddLinkedItems":{"errors":["Blocked work items are not available for this licensed feature"],"message":null}}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	err := client.AddLinks(context.Background(), 9001, []int64{9128}, apprelation.LinkTypeBlocks, "actor-token")
	if !errors.Is(err, apprelation.ErrFeatureUnavailable) {
		t.Fatalf("AddLinks() error = %v", err)
	}
}

func TestAddLinksValidatesTargetsThenUsesOneMutation(t *testing.T) {
	t.Parallel()
	var targetQueries, mutations int
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload.Query, "RelationshipTarget") {
			targetQueries++
			id := payload.Variables["id"].(string)
			if strings.HasSuffix(id, "/9128") {
				return response(http.StatusOK, `{"data":{"workItem":`+relationshipNodeJSON("9128", "128", "Issue", "First", "")+`}}`), nil
			}
			return response(http.StatusOK, `{"data":{"workItem":`+relationshipNodeJSON("9203", "203", "Task", "Second", "")+`}}`), nil
		}
		mutations++
		input := payload.Variables["input"].(map[string]any)
		ids := input["workItemsIds"].([]any)
		if len(ids) != 2 || ids[0] != "gid://gitlab/WorkItem/9128" || ids[1] != "gid://gitlab/WorkItem/9203" {
			t.Errorf("workItemsIds = %#v", ids)
		}
		return response(http.StatusOK, `{"data":{"workItemAddLinkedItems":{"errors":[],"message":null}}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	if err := client.AddLinks(context.Background(), 9001, []int64{9128, 9203}, apprelation.LinkTypeRelatesTo, "actor-token"); err != nil {
		t.Fatalf("AddLinks() error = %v", err)
	}
	if targetQueries != 2 || mutations != 1 {
		t.Fatalf("calls: targets=%d mutations=%d", targetQueries, mutations)
	}
}

func relationshipNodeJSON(id, iid, itemType, title, parentID string) string {
	parent := "null"
	if parentID != "" {
		parent = `{"id":"` + parentID + `"}`
	}
	return `{"id":"gid://gitlab/WorkItem/` + id + `","iid":"` + iid + `","title":"` + title + `","state":"OPEN","webUrl":"https://gitlab.example/work_items/` + iid + `","workItemType":{"name":"` + itemType + `"},"namespace":{"fullPath":"sitcon-tw/2027"},"widgets":[{"type":"STATUS","status":{"name":"To do","category":"to_do","color":"#737278"}},{"type":"ASSIGNEES","assignees":{"nodes":[{"id":"gid://gitlab/User/101","username":"alice","name":"Alice","avatarUrl":null,"webUrl":"https://gitlab.example/alice"}]}},{"type":"LABELS","labels":{"nodes":[{"title":"Backend","color":"#1D76DB","textColor":"#FFFFFF"}]}},{"type":"START_AND_DUE_DATE","startDate":"2026-08-28","dueDate":"2026-09-04"},{"type":"HIERARCHY","parent":` + parent + `}]}`
}
