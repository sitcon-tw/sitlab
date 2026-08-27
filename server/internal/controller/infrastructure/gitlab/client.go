package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appactivity "example.com/project-template/internal/controller/application/cardactivity"
	appoauth "example.com/project-template/internal/controller/application/oauth"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/domain/board"
	"example.com/project-template/internal/domain/directory"
	"example.com/project-template/internal/domain/identity"
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	ProjectPath  string
	AccessToken  string
}

func (c *Client) ProjectMembers(ctx context.Context) ([]directory.GitLabMember, error) {
	result := make([]directory.GitLabMember, 0)
	page := "1"
	for page != "" {
		requestURL := c.projectEndpoint("/members/all?per_page=100&page=") + url.QueryEscape(page)
		response, err := c.do(ctx, http.MethodGet, requestURL, nil, c.config.AccessToken, "")
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ID          int64  `json:"id"`
			Username    string `json:"username"`
			Name        string `json:"name"`
			AvatarURL   string `json:"avatar_url"`
			WebURL      string `json:"web_url"`
			AccessLevel int32  `json:"access_level"`
			State       string `json:"state"`
		}
		decodeErr := decodeJSON(response.Body, &rows)
		page = response.Header.Get("X-Next-Page")
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode GitLab members: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close GitLab members response: %w", closeErr)
		}
		for _, row := range rows {
			result = append(result, directory.GitLabMember{
				GitLabUserID: row.ID, Username: row.Username, DisplayName: row.Name,
				AvatarURL: row.AvatarURL, ProfileURL: row.WebURL,
				AccessLevel: row.AccessLevel, State: directory.MemberState(row.State),
			})
		}
	}
	return result, nil
}

func (c *Client) Issues(ctx context.Context) ([]appsync.GitLabIssue, error) {
	statuses, err := c.lifecycleStatuses(ctx)
	if err != nil {
		return nil, err
	}
	for _, required := range []string{"Waiting", "Inbox", "To do", "Doing", "Review", "Done"} {
		if _, ok := statuses[strings.ToLower(required)]; !ok {
			return nil, fmt.Errorf("GitLab lifecycle does not contain required status %q", required)
		}
	}
	result := make([]appsync.GitLabIssue, 0)
	var cursor *string
	for {
		var data workItemsData
		err := c.graphQL(ctx, workItemsQuery, map[string]any{"fullPath": c.config.ProjectPath, "after": cursor}, c.config.AccessToken, "", &data)
		if err != nil {
			return nil, err
		}
		for _, row := range data.Project.WorkItems.Nodes {
			result = append(result, mapWorkItem(row))
		}
		if !data.Project.WorkItems.PageInfo.HasNextPage {
			break
		}
		cursor = data.Project.WorkItems.PageInfo.EndCursor
	}
	return result, nil
}

func (c *Client) Issue(ctx context.Context, issueIID int64) (appsync.GitLabIssue, error) {
	var data workItemsData
	err := c.graphQL(ctx, workItemQuery, map[string]any{"fullPath": c.config.ProjectPath, "iids": []string{strconv.FormatInt(issueIID, 10)}}, c.config.AccessToken, "", &data)
	if err != nil {
		return appsync.GitLabIssue{}, err
	}
	if len(data.Project.WorkItems.Nodes) == 0 {
		return appsync.GitLabIssue{}, board.ErrCardNotFound
	}
	return mapWorkItem(data.Project.WorkItems.Nodes[0]), nil
}

func (c *Client) ProjectLabels(ctx context.Context) ([]appactivity.ProjectLabel, error) {
	result := make([]appactivity.ProjectLabel, 0)
	page := "1"
	for page != "" {
		requestURL := c.projectEndpoint("/labels?include_ancestor_groups=false&per_page=100&page=") + url.QueryEscape(page)
		response, err := c.do(ctx, http.MethodGet, requestURL, nil, c.config.AccessToken, "")
		if err != nil {
			return nil, err
		}
		var rows []labelWire
		decodeErr := decodeJSON(response.Body, &rows)
		page = response.Header.Get("X-Next-Page")
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode GitLab project labels: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close GitLab project labels response: %w", closeErr)
		}
		for _, row := range rows {
			if isDeprecatedLabel(row.Name) {
				continue
			}
			result = append(result, mapLabelWire(row))
		}
	}
	return result, nil
}

// Label writes use REST for all three operations. GitLab's GraphQL schema
// offers at most labelCreate — there is no labelUpdate or labelDelete — so
// rename and delete must be REST regardless, and splitting one CRUD triple
// across two transports buys nothing. REST also returns text_color, which the
// contract requires.
//
// All three run as the acting user: ARCHITECTURE.md already states mutations
// execute as the real actor, GitLab's own project role then decides who may
// manage a project-wide resource, and an irreversible change gets a real name
// in GitLab's audit trail. Reads keep the service token — the catalog is
// fetched on every drawer open and should not vary by the reader's role.
func (c *Client) CreateProjectLabel(ctx context.Context, write appactivity.ProjectLabelWrite, actorAccessToken string) (appactivity.ProjectLabel, error) {
	payload := map[string]any{"name": write.Name, "color": write.Color, "description": labelDescription(write.Description)}
	return c.writeProjectLabel(ctx, http.MethodPost, c.projectEndpoint("/labels"), payload, actorAccessToken)
}

func (c *Client) UpdateProjectLabel(ctx context.Context, labelID int64, write appactivity.ProjectLabelWrite, actorAccessToken string) (appactivity.ProjectLabel, error) {
	// GitLab renames through new_name; an empty description is how one is cleared.
	payload := map[string]any{"new_name": write.Name, "color": write.Color, "description": labelDescription(write.Description)}
	requestURL := c.projectEndpoint("/labels/") + strconv.FormatInt(labelID, 10)
	return c.writeProjectLabel(ctx, http.MethodPut, requestURL, payload, actorAccessToken)
}

func (c *Client) DeleteProjectLabel(ctx context.Context, labelID int64, actorAccessToken string) error {
	requestURL := c.projectEndpoint("/labels/") + strconv.FormatInt(labelID, 10)
	response, err := c.do(ctx, http.MethodDelete, requestURL, nil, "", actorAccessToken)
	if err != nil {
		return mapLabelStatus(err)
	}
	return response.Body.Close()
}

func (c *Client) writeProjectLabel(ctx context.Context, method, requestURL string, payload map[string]any, actorAccessToken string) (appactivity.ProjectLabel, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return appactivity.ProjectLabel{}, fmt.Errorf("encode GitLab project label: %w", err)
	}
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	response, err := c.doWithHeaders(ctx, method, requestURL, strings.NewReader(string(body)), "", actorAccessToken, headers)
	if err != nil {
		return appactivity.ProjectLabel{}, mapLabelStatus(err)
	}
	var row labelWire
	decodeErr := decodeJSON(response.Body, &row)
	closeErr := response.Body.Close()
	if decodeErr != nil {
		return appactivity.ProjectLabel{}, fmt.Errorf("decode GitLab project label: %w", decodeErr)
	}
	if closeErr != nil {
		return appactivity.ProjectLabel{}, fmt.Errorf("close GitLab project label response: %w", closeErr)
	}
	return mapLabelWire(row), nil
}

func labelDescription(description *string) string {
	if description == nil {
		return ""
	}
	return *description
}

func mapLabelWire(row labelWire) appactivity.ProjectLabel {
	return appactivity.ProjectLabel{ID: row.ID, Name: row.Name, Color: row.Color, TextColor: row.TextColor, Description: row.Description}
}

// mapLabelStatus is separate from mapActivityStatus on purpose: that one is
// shared with comment reads and must not start treating 400 as a validation
// failure.
func mapLabelStatus(err error) error {
	var statusError *httpStatusError
	if errors.As(err, &statusError) {
		switch statusError.status {
		case http.StatusConflict:
			return appactivity.ErrLabelConflict
		case http.StatusBadRequest:
			return appactivity.ErrLabelRejected
		}
	}
	return mapActivityStatus(err)
}

func (c *Client) Comments(ctx context.Context, issueIID int64, actorAccessToken string) ([]appactivity.Comment, error) {
	result := make([]appactivity.Comment, 0)
	page := "1"
	for page != "" {
		requestURL := c.projectEndpoint("/issues/") + strconv.FormatInt(issueIID, 10) + "/notes?order_by=created_at&sort=asc&per_page=100&page=" + url.QueryEscape(page)
		response, err := c.do(ctx, http.MethodGet, requestURL, nil, "", actorAccessToken)
		if err != nil {
			return nil, mapActivityStatus(err)
		}
		var rows []noteWire
		decodeErr := decodeJSON(response.Body, &rows)
		page = response.Header.Get("X-Next-Page")
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode GitLab issue notes: %w", decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close GitLab issue notes response: %w", closeErr)
		}
		for _, row := range rows {
			result = append(result, mapNoteWire(row))
		}
	}
	return result, nil
}

func (c *Client) CreateComment(ctx context.Context, issueIID int64, commentBody, actorAccessToken string) (appactivity.Comment, error) {
	payload, err := json.Marshal(map[string]string{"body": commentBody})
	if err != nil {
		return appactivity.Comment{}, fmt.Errorf("encode GitLab issue note: %w", err)
	}
	requestURL := c.projectEndpoint("/issues/") + strconv.FormatInt(issueIID, 10) + "/notes"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(payload)))
	if err != nil {
		return appactivity.Comment{}, fmt.Errorf("create GitLab issue note request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+actorAccessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return appactivity.Comment{}, identity.ErrGitLabUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 500 {
		return appactivity.Comment{}, identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return appactivity.Comment{}, mapActivityStatus(&httpStatusError{status: response.StatusCode})
	}
	var row noteWire
	if err := decodeJSON(response.Body, &row); err != nil {
		return appactivity.Comment{}, fmt.Errorf("decode GitLab issue note mutation: %w", err)
	}
	return mapNoteWire(row), nil
}

func (c *Client) ApplyIssue(ctx context.Context, mutation appsync.IssueMutation, actorAccessToken string) (appsync.GitLabIssue, error) {
	metadata, err := c.workItemMetadata(ctx, actorAccessToken)
	if err != nil {
		return appsync.GitLabIssue{}, err
	}
	if _, ok := metadata.statuses[strings.ToLower(mutation.GitLabStatusName)]; !ok {
		return appsync.GitLabIssue{}, fmt.Errorf("GitLab lifecycle does not contain required status %q", mutation.GitLabStatusName)
	}
	labelIDs := make([]string, 0, len(mutation.Labels))
	for _, label := range mutation.Labels {
		id, ok := metadata.labels[label]
		if !ok {
			// The worker replays labels captured on the optimistic row, so a
			// rename or delete landing while an operation is pending would
			// otherwise fail it permanently: retryOperation re-reads the same
			// stale names and fails identically. Drop the vanished label and
			// continue. The board never lets a user type a free-form label, so
			// the only ways here are that race and an out-of-band deletion.
			//
			// Team labels are the exception: issueTeam needs one to keep the
			// card on the board at all.
			if strings.HasPrefix(label, board.TeamLabelPrefix) {
				return appsync.GitLabIssue{}, fmt.Errorf("GitLab project label %q does not exist", label)
			}
			continue
		}
		labelIDs = append(labelIDs, id)
	}
	assigneeIDs := make([]string, 0, len(mutation.AssigneeGitLabUserIDs))
	for _, id := range mutation.AssigneeGitLabUserIDs {
		assigneeIDs = append(assigneeIDs, fmt.Sprintf("gid://gitlab/User/%d", id))
	}
	base := map[string]any{
		"title":                 mutation.Title,
		"descriptionWidget":     map[string]any{"description": mutation.Description},
		"assigneesWidget":       map[string]any{"assigneeIds": assigneeIDs},
		"startAndDueDateWidget": map[string]any{"startDate": nullableGraphQLDate(mutation.StartDate), "dueDate": nullableGraphQLDate(mutation.DueDate), "isFixed": true},
		"statusWidget":          map[string]any{"name": mutation.GitLabStatusName},
	}
	var data workItemMutationData
	if mutation.Create {
		base["namespacePath"] = c.config.ProjectPath
		base["workItemTypeId"] = metadata.issueTypeID
		base["labelsWidget"] = map[string]any{"labelIds": labelIDs}
		err = c.graphQL(ctx, workItemCreateMutation, map[string]any{"input": base}, "", actorAccessToken, &data)
	} else {
		current, currentErr := c.Issue(ctx, mutation.IssueIID)
		if currentErr != nil {
			return appsync.GitLabIssue{}, currentErr
		}
		base["id"] = fmt.Sprintf("gid://gitlab/WorkItem/%d", current.GitLabIssueID)
		add, remove := labelDelta(current.Labels, mutation.Labels, metadata.labels)
		base["labelsWidget"] = map[string]any{"addLabelIds": add, "removeLabelIds": remove}
		err = c.graphQL(ctx, workItemUpdateMutation, map[string]any{"input": base}, "", actorAccessToken, &data)
	}
	if err != nil {
		return appsync.GitLabIssue{}, err
	}
	payload := data.WorkItemCreate
	if !mutation.Create {
		payload = data.WorkItemUpdate
	}
	if len(payload.Errors) > 0 {
		return appsync.GitLabIssue{}, fmt.Errorf("GitLab work item mutation: %s", strings.Join(payload.Errors, "; "))
	}
	return mapWorkItem(payload.WorkItem), nil
}

type Client struct {
	http   *http.Client
	config Config
	base   *url.URL
}

func New(httpClient *http.Client, config Config) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid GitLab base URL")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{http: httpClient, config: config, base: base}, nil
}

func (c *Client) AuthorizationURL(state, codeChallenge string) string {
	values := url.Values{
		"client_id":             {c.config.ClientID},
		"redirect_uri":          {c.config.RedirectURI},
		"response_type":         {"code"},
		"scope":                 {"api"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return c.endpoint("/oauth/authorize") + "?" + values.Encode()
}

func (c *Client) ExchangeIdentity(ctx context.Context, code, verifier string) (appoauth.GitLabIdentity, error) {
	tokens, err := c.exchangeToken(ctx, code, verifier)
	if err != nil {
		return appoauth.GitLabIdentity{}, err
	}
	var user gitLabUser
	if err := c.get(ctx, c.endpoint("/api/v4/user"), tokens.AccessToken, &user); err != nil {
		return appoauth.GitLabIdentity{}, err
	}
	var member gitLabMember
	memberURL := c.endpoint("/api/v4/projects/") + url.PathEscape(c.config.ProjectPath) + "/members/all/" + strconv.FormatInt(user.ID, 10)
	if err := c.get(ctx, memberURL, tokens.AccessToken, &member); err != nil {
		var statusError *httpStatusError
		if errors.As(err, &statusError) && statusError.status == http.StatusNotFound {
			return appoauth.GitLabIdentity{}, identity.ErrProjectMemberRequired
		}
		return appoauth.GitLabIdentity{}, err
	}
	if member.State != "active" || member.AccessLevel <= 0 {
		return appoauth.GitLabIdentity{}, identity.ErrProjectMemberRequired
	}
	return appoauth.GitLabIdentity{
		GitLabUserID: user.ID, Username: user.Username, DisplayName: user.Name,
		AvatarURL: user.AvatarURL, ProfileURL: user.WebURL,
		AccessLevel: member.AccessLevel, State: member.State, Tokens: tokens,
	}, nil
}

func (c *Client) exchangeToken(ctx context.Context, code, verifier string) (appoauth.OAuthTokens, error) {
	values := url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.config.RedirectURI},
		"code_verifier": {verifier},
	}
	return c.requestOAuthToken(ctx, values)
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (appoauth.OAuthTokens, error) {
	return c.requestOAuthToken(ctx, url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
		"redirect_uri":  {c.config.RedirectURI},
	})
}

func (c *Client) requestOAuthToken(ctx context.Context, values url.Values) (appoauth.OAuthTokens, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/oauth/token"), strings.NewReader(values.Encode()))
	if err != nil {
		return appoauth.OAuthTokens{}, fmt.Errorf("create GitLab token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return appoauth.OAuthTokens{}, fmt.Errorf("%w: exchange OAuth token", identity.ErrGitLabUnavailable)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 500 {
		return appoauth.OAuthTokens{}, identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return appoauth.OAuthTokens{}, &httpStatusError{status: response.StatusCode}
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		CreatedAt    int64  `json:"created_at"`
	}
	if err := decodeJSON(response.Body, &token); err != nil {
		return appoauth.OAuthTokens{}, fmt.Errorf("decode GitLab token: %w", err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.ExpiresIn <= 0 {
		return appoauth.OAuthTokens{}, fmt.Errorf("GitLab token response is incomplete")
	}
	issuedAt := time.Now().UTC()
	if token.CreatedAt > 0 {
		issuedAt = time.Unix(token.CreatedAt, 0).UTC()
	}
	return appoauth.OAuthTokens{
		AccessToken: token.AccessToken, RefreshToken: token.RefreshToken,
		ExpiresAt: issuedAt.Add(time.Duration(token.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) get(ctx context.Context, requestURL, accessToken string, target any) error {
	response, err := c.do(ctx, http.MethodGet, requestURL, nil, "", accessToken)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 500 {
		return identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &httpStatusError{status: response.StatusCode}
	}
	if err := decodeJSON(response.Body, target); err != nil {
		return fmt.Errorf("decode GitLab response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, requestURL string, body io.Reader, privateToken, bearerToken string) (*http.Response, error) {
	return c.doWithHeaders(ctx, method, requestURL, body, privateToken, bearerToken, nil)
}

func (c *Client) doWithHeaders(ctx context.Context, method, requestURL string, body io.Reader, privateToken, bearerToken string, headers http.Header) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("create GitLab request: %w", err)
	}
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	if privateToken != "" {
		request.Header.Set("PRIVATE-TOKEN", privateToken)
	}
	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, identity.ErrGitLabUnavailable
	}
	if response.StatusCode >= 500 {
		_ = response.Body.Close()
		return nil, identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, &httpStatusError{status: response.StatusCode}
	}
	return response, nil
}

func (c *Client) endpoint(path string) string {
	return strings.TrimRight(c.base.String(), "/") + path
}

func (c *Client) projectEndpoint(path string) string {
	return c.endpoint("/api/v4/projects/") + url.PathEscape(c.config.ProjectPath) + path
}

func decodeJSON(reader io.Reader, target any) error {
	return json.NewDecoder(io.LimitReader(reader, 2<<20)).Decode(target)
}

func (c *Client) graphQL(ctx context.Context, query string, variables map[string]any, privateToken, bearerToken string, target any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("encode GitLab GraphQL request: %w", err)
	}
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	response, err := c.doWithHeaders(ctx, http.MethodPost, c.endpoint("/api/graphql"), strings.NewReader(string(body)), privateToken, bearerToken, headers)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := decodeJSON(response.Body, &envelope); err != nil {
		return fmt.Errorf("decode GitLab GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, item := range envelope.Errors {
			messages = append(messages, item.Message)
		}
		return fmt.Errorf("GitLab GraphQL: %s", strings.Join(messages, "; "))
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode GitLab GraphQL data: %w", err)
	}
	return nil
}

const workItemFields = `
  id iid title state webUrl createdAt updatedAt
  widgets {
    type
    ... on WorkItemWidgetStatus { status { id name category color } }
    ... on WorkItemWidgetLabels { labels { nodes { id title } } }
    ... on WorkItemWidgetAssignees { assignees { nodes { id username } } }
    ... on WorkItemWidgetStartAndDueDate { startDate dueDate }
    ... on WorkItemWidgetDescription { description }
  }`

const workItemsQuery = `query WorkItems($fullPath: ID!, $after: String) {
  project(fullPath: $fullPath) {
    workItems(first: 100, after: $after, types: [ISSUE], sort: UPDATED_DESC) {
      pageInfo { hasNextPage endCursor }
      nodes {` + workItemFields + `}
    }
  }
}`

const workItemQuery = `query WorkItem($fullPath: ID!, $iids: [String!]) {
  project(fullPath: $fullPath) {
    workItems(first: 1, iids: $iids, types: [ISSUE]) { nodes {` + workItemFields + `} }
  }
}`

const workItemCreateMutation = `mutation CreateWorkItem($input: WorkItemCreateInput!) {
  workItemCreate(input: $input) { errors workItem {` + workItemFields + `} }
}`

const workItemUpdateMutation = `mutation UpdateWorkItem($input: WorkItemUpdateInput!) {
  workItemUpdate(input: $input) { errors workItem {` + workItemFields + `} }
}`

const workItemMetadataQuery = `query WorkItemMetadata($fullPath: ID!, $after: String) {
  project(fullPath: $fullPath) {
    labels(first: 100, after: $after) { pageInfo { hasNextPage endCursor } nodes { id title } }
    workItemTypes { nodes {
      id name
      widgetDefinitions {
        type
        ... on WorkItemWidgetDefinitionStatus { allowedStatuses { id name category color } }
      }
    } }
  }
}`

const workItemLifecycleQuery = `query WorkItemLifecycle($fullPath: ID!) {
  project(fullPath: $fullPath) {
    workItemTypes { nodes {
      name
      widgetDefinitions {
        type
        ... on WorkItemWidgetDefinitionStatus { allowedStatuses { id name } }
      }
    } }
  }
}`

type workItemNode struct {
	ID        string    `json:"id"`
	IID       string    `json:"iid"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	WebURL    string    `json:"webUrl"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Widgets   []struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		StartDate   string `json:"startDate"`
		DueDate     string `json:"dueDate"`
		Status      *struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Color    string `json:"color"`
		} `json:"status"`
		Labels struct {
			Nodes []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"nodes"`
		} `json:"labels"`
		Assignees struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"assignees"`
	} `json:"widgets"`
}

type workItemsData struct {
	Project struct {
		WorkItems struct {
			PageInfo struct {
				HasNextPage bool    `json:"hasNextPage"`
				EndCursor   *string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []workItemNode `json:"nodes"`
		} `json:"workItems"`
	} `json:"project"`
}

type workItemMutationPayload struct {
	Errors   []string     `json:"errors"`
	WorkItem workItemNode `json:"workItem"`
}

type workItemMutationData struct {
	WorkItemCreate workItemMutationPayload `json:"workItemCreate"`
	WorkItemUpdate workItemMutationPayload `json:"workItemUpdate"`
}

func mapWorkItem(row workItemNode) appsync.GitLabIssue {
	iid, _ := strconv.ParseInt(row.IID, 10, 64)
	id := parseGlobalID(row.ID)
	issue := appsync.GitLabIssue{
		IssueIID: iid, GitLabIssueID: id, Title: row.Title, WebURL: row.WebURL,
		State: strings.ToLower(row.State), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
	for _, widget := range row.Widgets {
		switch widget.Type {
		case "DESCRIPTION":
			issue.Description = widget.Description
		case "STATUS":
			if widget.Status != nil {
				issue.GitLabStatusName = widget.Status.Name
			}
		case "LABELS":
			for _, label := range widget.Labels.Nodes {
				issue.Labels = append(issue.Labels, label.Title)
			}
		case "ASSIGNEES":
			for _, assignee := range widget.Assignees.Nodes {
				issue.AssigneeGitLabUserIDs = append(issue.AssigneeGitLabUserIDs, parseGlobalID(assignee.ID))
			}
		case "START_AND_DUE_DATE":
			issue.StartDate, issue.DueDate = widget.StartDate, widget.DueDate
		}
	}
	return issue
}

func parseGlobalID(value string) int64 {
	parts := strings.Split(value, "/")
	if len(parts) == 0 {
		return 0
	}
	result, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return result
}

type workItemMetadata struct {
	issueTypeID string
	labels      map[string]string
	statuses    map[string]string
}

func (c *Client) lifecycleStatuses(ctx context.Context) (map[string]string, error) {
	var data struct {
		Project struct {
			WorkItemTypes struct {
				Nodes []struct {
					Name              string
					WidgetDefinitions []struct {
						AllowedStatuses []struct{ ID, Name string } `json:"allowedStatuses"`
					} `json:"widgetDefinitions"`
				} `json:"nodes"`
			} `json:"workItemTypes"`
		} `json:"project"`
	}
	if err := c.graphQL(ctx, workItemLifecycleQuery, map[string]any{"fullPath": c.config.ProjectPath}, c.config.AccessToken, "", &data); err != nil {
		return nil, err
	}
	statuses := make(map[string]string)
	for _, itemType := range data.Project.WorkItemTypes.Nodes {
		if !strings.EqualFold(itemType.Name, "Issue") {
			continue
		}
		for _, widget := range itemType.WidgetDefinitions {
			for _, status := range widget.AllowedStatuses {
				statuses[strings.ToLower(status.Name)] = status.ID
			}
		}
	}
	return statuses, nil
}

func (c *Client) workItemMetadata(ctx context.Context, actorAccessToken string) (workItemMetadata, error) {
	return c.workItemMetadataWithCredentials(ctx, "", actorAccessToken)
}

func (c *Client) workItemMetadataWithCredentials(ctx context.Context, privateToken, bearerToken string) (workItemMetadata, error) {
	var data struct {
		Project struct {
			Labels struct {
				PageInfo struct {
					HasNextPage bool    `json:"hasNextPage"`
					EndCursor   *string `json:"endCursor"`
				} `json:"pageInfo"`
				Nodes []struct{ ID, Title string }
			} `json:"labels"`
			WorkItemTypes struct {
				Nodes []struct {
					ID                string
					Name              string
					WidgetDefinitions []struct {
						AllowedStatuses []struct{ ID, Name string } `json:"allowedStatuses"`
					} `json:"widgetDefinitions"`
				} `json:"nodes"`
			} `json:"workItemTypes"`
		} `json:"project"`
	}
	metadata := workItemMetadata{labels: make(map[string]string), statuses: make(map[string]string)}
	var cursor *string
	for {
		data = struct {
			Project struct {
				Labels struct {
					PageInfo struct {
						HasNextPage bool    `json:"hasNextPage"`
						EndCursor   *string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct{ ID, Title string }
				} `json:"labels"`
				WorkItemTypes struct {
					Nodes []struct {
						ID                string
						Name              string
						WidgetDefinitions []struct {
							AllowedStatuses []struct{ ID, Name string } `json:"allowedStatuses"`
						} `json:"widgetDefinitions"`
					} `json:"nodes"`
				} `json:"workItemTypes"`
			} `json:"project"`
		}{}
		if err := c.graphQL(ctx, workItemMetadataQuery, map[string]any{"fullPath": c.config.ProjectPath, "after": cursor}, privateToken, bearerToken, &data); err != nil {
			return workItemMetadata{}, err
		}
		for _, label := range data.Project.Labels.Nodes {
			metadata.labels[label.Title] = label.ID
		}
		for _, itemType := range data.Project.WorkItemTypes.Nodes {
			if strings.EqualFold(itemType.Name, "Issue") {
				metadata.issueTypeID = itemType.ID
				for _, widget := range itemType.WidgetDefinitions {
					for _, status := range widget.AllowedStatuses {
						metadata.statuses[strings.ToLower(status.Name)] = status.ID
					}
				}
			}
		}
		if !data.Project.Labels.PageInfo.HasNextPage {
			break
		}
		cursor = data.Project.Labels.PageInfo.EndCursor
	}
	if metadata.issueTypeID == "" {
		return workItemMetadata{}, fmt.Errorf("GitLab Issue work item type is unavailable")
	}
	return metadata, nil
}

func labelDelta(current, desired []string, ids map[string]string) ([]string, []string) {
	want := make(map[string]struct{}, len(desired))
	for _, label := range desired {
		want[label] = struct{}{}
	}
	have := make(map[string]struct{}, len(current))
	for _, label := range current {
		have[label] = struct{}{}
	}
	add := make([]string, 0)
	for _, label := range desired {
		if _, ok := have[label]; !ok {
			add = append(add, ids[label])
		}
	}
	remove := make([]string, 0)
	for _, label := range current {
		if _, ok := want[label]; !ok {
			if id, exists := ids[label]; exists {
				remove = append(remove, id)
			}
		}
	}
	return add, remove
}

func nullableGraphQLDate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func isDeprecatedLabel(label string) bool {
	return board.DeprecatedLabel(label)
}

type httpStatusError struct{ status int }

func (e *httpStatusError) Error() string { return fmt.Sprintf("GitLab returned HTTP %d", e.status) }

type gitLabUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

type gitLabMember struct {
	AccessLevel int32  `json:"access_level"`
	State       string `json:"state"`
}

type labelWire struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	TextColor   string  `json:"text_color"`
	Description *string `json:"description"`
}

type noteWire struct {
	ID        int64      `json:"id"`
	Body      string     `json:"body"`
	Author    gitLabUser `json:"author"`
	System    bool       `json:"system"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func mapNoteWire(row noteWire) appactivity.Comment {
	return appactivity.Comment{
		ID: row.ID, Body: row.Body, System: row.System,
		Author: appactivity.CommentAuthor{
			GitLabUserID: row.Author.ID, Username: row.Author.Username, DisplayName: row.Author.Name,
			AvatarURL: row.Author.AvatarURL, ProfileURL: row.Author.WebURL,
		},
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func mapActivityStatus(err error) error {
	var statusError *httpStatusError
	if !errors.As(err, &statusError) {
		return err
	}
	switch statusError.status {
	case http.StatusNotFound:
		return board.ErrCardNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return appactivity.ErrGitLabForbidden
	default:
		return err
	}
}
