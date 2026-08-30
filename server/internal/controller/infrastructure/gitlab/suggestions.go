package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	appactivity "example.com/project-template/internal/controller/application/cardactivity"
)

const suggestionLimit = 20

// QuickActionSuggestions normalizes GitLab's separate autocomplete resources
// behind one stable application contract. Adding a new parameter provider stays a
// single switch case and never changes the editor component.
func (c *Client) QuickActionSuggestions(ctx context.Context, kind, query string, _ int64, actorAccessToken string) ([]appactivity.QuickActionSuggestion, error) {
	switch kind {
	case "member":
		var rows []struct {
			ID          int64  `json:"id"`
			AccessLevel int64  `json:"access_level"`
			Username    string `json:"username"`
			Name        string `json:"name"`
			AvatarURL   string `json:"avatar_url"`
			WebURL      string `json:"web_url"`
			State       string `json:"state"`
		}
		if err := c.suggestionGET(ctx, c.projectEndpoint("/members/all"), url.Values{"query": {query}}, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			if row.State != "" && row.State != "active" {
				continue
			}
			items = append(items, suggestion("member", strconv.FormatInt(row.ID, 10), "@"+row.Username, "@"+row.Username, row.Name, row.AvatarURL, ""))
		}
		return items, nil
	case "label":
		var rows []labelWire
		if err := c.suggestionGET(ctx, c.projectEndpoint("/labels"), url.Values{"search": {query}, "include_ancestor_groups": {"true"}}, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			detail := "GitLab label"
			if row.Description != nil && *row.Description != "" {
				detail = *row.Description
			}
			items = append(items, suggestion("label", strconv.FormatInt(row.ID, 10), "~"+quotedReference(row.Name), row.Name, detail, "", row.Color))
		}
		return items, nil
	case "work_item":
		return c.workItemSuggestions(ctx, query, actorAccessToken)
	case "merge_request":
		var rows []struct {
			IID           int64 `json:"iid"`
			Title, WebURL string
		}
		values := url.Values{"scope": {"all"}, "search": {query}}
		if numeric, err := strconv.ParseInt(query, 10, 64); err == nil && numeric > 0 {
			values.Del("search")
			values["iids[]"] = []string{query}
		}
		if err := c.suggestionGET(ctx, c.projectEndpoint("/merge_requests"), values, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			ref := "!" + strconv.FormatInt(row.IID, 10)
			items = append(items, suggestion("merge_request", strconv.FormatInt(row.IID, 10), ref, ref, row.Title, "", ""))
		}
		return items, nil
	case "milestone":
		var rows []struct {
			ID                 int64
			Title, Description string
		}
		if err := c.suggestionGET(ctx, c.projectEndpoint("/milestones"), url.Values{"search": {query}, "state": {"active"}}, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			items = append(items, suggestion("milestone", strconv.FormatInt(row.ID, 10), "%"+quotedReference(row.Title), row.Title, row.Description, "", ""))
		}
		return items, nil
	case "iteration":
		var rows []struct {
			ID          int64  `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			StartDate   string `json:"start_date"`
			DueDate     string `json:"due_date"`
		}
		if err := c.suggestionGET(ctx, c.projectEndpoint("/iterations"), url.Values{"search": {query}, "state": {"all"}}, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			if row.Title == "" {
				continue
			}
			detail := strings.Trim(strings.Join([]string{row.StartDate, row.DueDate}, " – "), " –")
			items = append(items, suggestion("iteration", strconv.FormatInt(row.ID, 10), "*iteration:"+quotedReference(row.Title), row.Title, detail, "", ""))
		}
		return items, nil
	case "snippet":
		var rows []struct {
			ID                 int64
			Title, Description string
		}
		if err := c.suggestionGET(ctx, c.projectEndpoint("/snippets"), nil, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		needle := strings.ToLower(query)
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			if needle != "" && !strings.Contains(strings.ToLower(row.Title+" "+row.Description), needle) {
				continue
			}
			ref := "$" + strconv.FormatInt(row.ID, 10)
			items = append(items, suggestion("snippet", strconv.FormatInt(row.ID, 10), ref, ref+" "+row.Title, row.Description, "", ""))
			if len(items) == suggestionLimit {
				break
			}
		}
		return items, nil
	case "branch":
		var rows []struct{ Name string }
		if err := c.suggestionGET(ctx, c.projectEndpoint("/repository/branches"), url.Values{"search": {query}}, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			items = append(items, suggestion("branch", row.Name, row.Name, row.Name, "GitLab branch", "", ""))
		}
		return items, nil
	case "project":
		var rows []struct {
			ID                int64  `json:"id"`
			PathWithNamespace string `json:"path_with_namespace"`
			NameWithNamespace string `json:"name_with_namespace"`
			WebURL            string `json:"web_url"`
		}
		if err := c.suggestionGET(ctx, c.endpoint("/api/v4/projects"), url.Values{"membership": {"true"}, "simple": {"true"}, "search": {query}}, actorAccessToken, &rows); err != nil {
			return nil, err
		}
		items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
		for _, row := range rows {
			items = append(items, suggestion("project", strconv.FormatInt(row.ID, 10), row.PathWithNamespace, row.PathWithNamespace, row.NameWithNamespace, "", ""))
		}
		return items, nil
	case "epic":
		return c.epicSuggestions(ctx, query, actorAccessToken)
	default:
		return nil, fmt.Errorf("unsupported GitLab autocomplete source %q", kind)
	}
}

func (c *Client) workItemSuggestions(ctx context.Context, query, actorAccessToken string) ([]appactivity.QuickActionSuggestion, error) {
	var rows []struct {
		IID   int64 `json:"iid"`
		Title string
	}
	values := url.Values{"scope": {"all"}, "search": {query}}
	if numeric, err := strconv.ParseInt(query, 10, 64); err == nil && numeric > 0 {
		values.Del("search")
		values["iids[]"] = []string{query}
	}
	if err := c.suggestionGET(ctx, c.projectEndpoint("/issues"), values, actorAccessToken, &rows); err != nil {
		return nil, err
	}
	items := make([]appactivity.QuickActionSuggestion, 0, len(rows))
	for _, row := range rows {
		ref := "#" + strconv.FormatInt(row.IID, 10)
		items = append(items, suggestion("work_item", strconv.FormatInt(row.IID, 10), ref, ref, row.Title, "", ""))
	}
	return items, nil
}

const epicSuggestionsQuery = `query QuickActionEpics($fullPath: ID!, $search: String) {
  group(fullPath: $fullPath) {
    workItems(first: 20, search: $search, types: [EPIC]) { nodes { id iid title } }
  }
}`

func (c *Client) epicSuggestions(ctx context.Context, query, actorAccessToken string) ([]appactivity.QuickActionSuggestion, error) {
	separator := strings.LastIndex(c.config.ProjectPath, "/")
	if separator < 1 {
		return []appactivity.QuickActionSuggestion{}, nil
	}
	groupPath := c.config.ProjectPath[:separator]
	var data struct {
		Group struct {
			WorkItems struct {
				Nodes []struct{ ID, IID, Title string }
			}
		}
	}
	if err := c.graphQL(ctx, epicSuggestionsQuery, map[string]any{"fullPath": groupPath, "search": query}, "", actorAccessToken, &data); err != nil {
		return nil, err
	}
	items := make([]appactivity.QuickActionSuggestion, 0, len(data.Group.WorkItems.Nodes))
	for _, row := range data.Group.WorkItems.Nodes {
		ref := "&" + row.IID
		items = append(items, suggestion("epic", row.ID, ref, ref, row.Title, "", ""))
	}
	return items, nil
}

func (c *Client) suggestionGET(ctx context.Context, endpoint string, values url.Values, actorAccessToken string, target any) error {
	if values == nil {
		values = make(url.Values)
	}
	values.Set("per_page", strconv.Itoa(suggestionLimit))
	requestURL := endpoint
	if encoded := values.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}
	response, err := c.do(ctx, http.MethodGet, requestURL, nil, "", actorAccessToken)
	if err != nil {
		return mapActivityStatus(err)
	}
	defer func() { _ = response.Body.Close() }()
	if err := decodeJSON(response.Body, target); err != nil {
		return fmt.Errorf("decode GitLab autocomplete source: %w", err)
	}
	return nil
}

func suggestion(kind, id, value, label, detail, avatarURL, color string) appactivity.QuickActionSuggestion {
	return appactivity.QuickActionSuggestion{ID: id, Kind: kind, Value: value, Label: label, Detail: detail, AvatarURL: avatarURL, Color: color}
}

func quotedReference(value string) string {
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
