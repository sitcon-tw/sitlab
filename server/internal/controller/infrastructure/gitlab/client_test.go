package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"example.com/project-template/internal/controller/application/cardactivity"
	"example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/identity"
)

func TestSnapshotEndpointsParseMembersAndIssues(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.Header.Get("PRIVATE-TOKEN") != "project-token" {
			t.Errorf("PRIVATE-TOKEN = %q", request.Header.Get("PRIVATE-TOKEN"))
		}
		switch {
		case strings.Contains(request.URL.Path, "/members/all"):
			return response(http.StatusOK, `[{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice","access_level":40,"state":"active"}]`), nil
		case request.URL.Path == "/api/graphql":
			var payload struct {
				Query     string         `json:"query"`
				Variables map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(payload.Query, "WorkItemMetadata") {
				return response(http.StatusOK, `{"data":{"project":{"labels":{"pageInfo":{"hasNextPage":false},"nodes":[{"id":"gid://gitlab/ProjectLabel/1","title":"Team::開發組"}]},"workItemTypes":{"nodes":[{"id":"gid://gitlab/WorkItems::Type/1","name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`), nil
			}
			if strings.Contains(payload.Query, "WorkItemLifecycle") {
				return response(http.StatusOK, `{"data":{"project":{"workItemTypes":{"nodes":[{"name":"Issue","widgetDefinitions":[{"allowedStatuses":[{"id":"status-1","name":"Waiting"},{"id":"status-2","name":"Inbox"},{"id":"status-3","name":"To do"},{"id":"status-4","name":"Doing"},{"id":"status-5","name":"Review"},{"id":"status-6","name":"Done"}]}]}]}}}}`), nil
			}
			if strings.Contains(payload.Query, "CreateWorkItem") {
				if request.Header.Get("Authorization") != "Bearer actor-token" {
					t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
				}
				input := payload.Variables["input"].(map[string]any)
				if input["title"] != "[開發組] 新卡" || input["statusWidget"].(map[string]any)["name"] != "To do" {
					t.Errorf("work item input = %#v", input)
				}
				return response(http.StatusOK, `{"data":{"workItemCreate":{"errors":[],"workItem":`+testWorkItemJSON("2", "20", "[開發組] 新卡")+`}}}`), nil
			}
			if strings.Contains(payload.Query, "query WorkItems") || strings.Contains(payload.Query, "query WorkItem") {
				return response(http.StatusOK, `{"data":{"project":{"workItems":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[`+testWorkItemJSON("1", "10", "[開發組] 修正流程")+`]}}}}`), nil
			}
			return response(http.StatusBadRequest, `{}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, err := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027",
		AccessToken: "project-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	members, err := client.ProjectMembers(context.Background())
	if err != nil || len(members) != 1 || members[0].GitLabUserID != 101 {
		t.Fatalf("ProjectMembers() = %#v, %v", members, err)
	}
	issues, err := client.Issues(context.Background())
	if err != nil || len(issues) != 1 || len(issues[0].AssigneeGitLabUserIDs) != 2 || issues[0].Description != "工作拆解" || issues[0].StartDate != "2026-07-17" {
		t.Fatalf("Issues() = %#v, %v", issues, err)
	}
	issue, err := client.Issue(context.Background(), 1)
	if err != nil || issue.IssueIID != 1 || issue.Description != "工作拆解" {
		t.Fatalf("Issue() = %#v, %v", issue, err)
	}
	created, err := client.ApplyIssue(context.Background(), sync.IssueMutation{
		Create: true, Title: "[開發組] 新卡", Description: "詳細規劃", StartDate: "2026-07-17", DueDate: "2026-07-21",
		Labels: []string{"Team::開發組"}, AssigneeGitLabUserIDs: []int64{101, 202}, GitLabStatusName: "To do",
	}, "actor-token")
	if err != nil || created.IssueIID != 2 || created.StartDate != "2026-07-17" {
		t.Fatalf("ApplyIssue() = %#v, %v", created, err)
	}
}

func TestMissingIssueMapsToCardNotFound(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"data":{"project":{"workItems":{"nodes":[]}}}}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})
	_, err := client.Issue(context.Background(), 404)
	if !errors.Is(err, board.ErrCardNotFound) {
		t.Fatalf("Issue() error = %v", err)
	}
}

func TestProjectLabelsAndCommentsUseExpectedCredentialsAndPagination(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/labels"):
			if request.Header.Get("PRIVATE-TOKEN") != "project-token" {
				t.Errorf("labels PRIVATE-TOKEN = %q", request.Header.Get("PRIVATE-TOKEN"))
			}
			return response(http.StatusOK, `[{"name":"Backend","color":"#1D76DB","text_color":"#FFFFFF","description":"Server work"}]`), nil
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/notes"):
			assertBearer(t, request)
			if request.URL.Query().Get("page") == "1" {
				result := response(http.StatusOK, `[{"id":2,"body":"changed status","system":true,"created_at":"2026-07-29T09:00:00Z","updated_at":"2026-07-29T09:00:00Z","author":{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice"}}]`)
				result.Header.Set("X-Next-Page", "2")
				return result, nil
			}
			return response(http.StatusOK, `[{"id":3,"body":"please review","system":false,"created_at":"2026-07-29T10:00:00Z","updated_at":"2026-07-29T10:00:00Z","author":{"id":102,"username":"bob","name":"Bob","web_url":"https://gitlab.example/bob"}}]`), nil
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/notes"):
			assertBearer(t, request)
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil || payload["body"] != "new comment" {
				t.Fatalf("comment payload = %#v, %v", payload, err)
			}
			return response(http.StatusCreated, `{"id":4,"body":"new comment","system":false,"created_at":"2026-07-29T11:00:00Z","updated_at":"2026-07-29T11:00:00Z","author":{"id":101,"username":"alice","name":"Alice","web_url":"https://gitlab.example/alice"}}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, _ := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027", AccessToken: "project-token",
	})
	labels, err := client.ProjectLabels(context.Background())
	if err != nil || len(labels) != 1 || labels[0].Name != "Backend" || labels[0].TextColor != "#FFFFFF" {
		t.Fatalf("ProjectLabels() = %#v, %v", labels, err)
	}
	comments, err := client.Comments(context.Background(), 42, "token")
	if err != nil || len(comments) != 2 || !comments[0].System || comments[1].Author.DisplayName != "Bob" {
		t.Fatalf("Comments() = %#v, %v", comments, err)
	}
	created, err := client.CreateComment(context.Background(), 42, "new comment", "token")
	if err != nil || created.ID != 4 || created.Body != "new comment" {
		t.Fatalf("CreateComment() = %#v, %v", created, err)
	}
}

func TestCommentForbiddenMapsToActivityError(t *testing.T) {
	t.Parallel()
	client, _ := New(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, `{}`), nil
	})}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	_, err := client.Comments(context.Background(), 42, "token")
	if !errors.Is(err, cardactivity.ErrGitLabForbidden) {
		t.Fatalf("Comments() error = %v", err)
	}
}

func TestOAuthAndProjectMembership(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/oauth/token":
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.FormValue("code_verifier") != "verifier" {
				t.Errorf("code_verifier = %q", request.FormValue("code_verifier"))
			}
			return response(http.StatusOK, `{"access_token":"token","refresh_token":"refresh","expires_in":7200,"created_at":10000}`), nil
		case "/api/v4/user":
			assertBearer(t, request)
			return response(http.StatusOK, `{"id":123,"username":"yorukot","name":"Yorukot","avatar_url":"https://img.example/avatar.png","web_url":"https://gitlab.com/yorukot"}`), nil
		case "/api/v4/projects/sitcon-tw/2027/members/all/123":
			if request.URL.EscapedPath() != "/api/v4/projects/sitcon-tw%2F2027/members/all/123" {
				t.Errorf("escaped project path = %q", request.URL.EscapedPath())
			}
			assertBearer(t, request)
			return response(http.StatusOK, `{"access_level":40,"state":"active"}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, err := New(&http.Client{Transport: transport}, Config{
		BaseURL: "https://gitlab.example", ClientID: "client", ClientSecret: "secret",
		RedirectURI: "https://board.example/callback", ProjectPath: "sitcon-tw/2027",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ExchangeIdentity(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatalf("ExchangeIdentity() error = %v", err)
	}
	if result.GitLabUserID != 123 || result.AccessLevel != 40 || result.Username != "yorukot" {
		t.Fatalf("ExchangeIdentity() = %#v", result)
	}
	authorize, err := url.Parse(client.AuthorizationURL("state", "challenge"))
	if err != nil {
		t.Fatal(err)
	}
	if authorize.Query().Get("state") != "state" || authorize.Query().Get("code_challenge_method") != "S256" || authorize.Query().Get("scope") != "api" {
		t.Fatalf("AuthorizationURL() = %s", authorize)
	}
}

func TestMissingProjectMemberIsForbidden(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Path == "/oauth/token":
			return response(http.StatusOK, `{"access_token":"token","refresh_token":"refresh","expires_in":7200}`), nil
		case request.URL.Path == "/api/v4/user":
			return response(http.StatusOK, `{"id":123,"username":"outside","name":"Outside"}`), nil
		case strings.Contains(request.URL.Path, "/members/all/"):
			return response(http.StatusNotFound, `{}`), nil
		default:
			return response(http.StatusNotFound, `{}`), nil
		}
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ProjectPath: "sitcon-tw/2027"})
	_, err := client.ExchangeIdentity(context.Background(), "code", "verifier")
	if !errors.Is(err, identity.ErrProjectMemberRequired) {
		t.Fatalf("ExchangeIdentity() error = %v", err)
	}
}

func TestRefreshTokenRotatesOAuthCredential(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.FormValue("grant_type") != "refresh_token" || request.FormValue("refresh_token") != "old-refresh" {
			t.Fatalf("refresh form = %#v", request.Form)
		}
		return response(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":7200,"created_at":10000}`), nil
	})
	client, _ := New(&http.Client{Transport: transport}, Config{BaseURL: "https://gitlab.example", ClientID: "client", ClientSecret: "secret"})

	tokens, err := client.RefreshToken(context.Background(), "old-refresh")
	if err != nil || tokens.AccessToken != "new-access" || tokens.RefreshToken != "new-refresh" || !tokens.ExpiresAt.Equal(time.Unix(17_200, 0).UTC()) {
		t.Fatalf("RefreshToken() = %#v, %v", tokens, err)
	}
}

func assertBearer(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func testWorkItemJSON(iid, id, title string) string {
	value, _ := json.Marshal(map[string]any{
		"id":        "gid://gitlab/WorkItem/" + id,
		"iid":       iid,
		"title":     title,
		"state":     "OPEN",
		"webUrl":    "https://gitlab.example/issues/" + iid,
		"createdAt": "2026-07-13T08:00:00Z",
		"updatedAt": "2026-07-14T08:00:00Z",
		"widgets": []any{
			map[string]any{"type": "DESCRIPTION", "description": "工作拆解"},
			map[string]any{"type": "STATUS", "status": map[string]any{"id": "gid://gitlab/WorkItems::Statuses::Custom::Status/1513", "name": "To do", "category": "to_do", "color": "#ed9121"}},
			map[string]any{"type": "LABELS", "labels": map[string]any{"nodes": []any{map[string]any{"id": "gid://gitlab/ProjectLabel/1", "title": "Team::開發組"}}}},
			map[string]any{"type": "ASSIGNEES", "assignees": map[string]any{"nodes": []any{map[string]any{"id": "gid://gitlab/User/101"}, map[string]any{"id": "gid://gitlab/User/202"}}}},
			map[string]any{"type": "START_AND_DUE_DATE", "startDate": "2026-07-17", "dueDate": "2026-07-21"},
		},
	})
	return string(value)
}
