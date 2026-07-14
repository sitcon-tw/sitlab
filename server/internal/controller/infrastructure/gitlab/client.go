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

	appoauth "example.com/project-template/internal/controller/application/oauth"
	appsync "example.com/project-template/internal/controller/application/sync"
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
		response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode GitLab members: %w", decodeErr)
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
	result := make([]appsync.GitLabIssue, 0)
	page := "1"
	for page != "" {
		requestURL := c.projectEndpoint("/issues?scope=all&state=all&order_by=updated_at&sort=desc&per_page=100&page=") + url.QueryEscape(page)
		response, err := c.do(ctx, http.MethodGet, requestURL, nil, c.config.AccessToken, "")
		if err != nil {
			return nil, err
		}
		var rows []struct {
			ID        int64     `json:"id"`
			IID       int64     `json:"iid"`
			Title     string    `json:"title"`
			WebURL    string    `json:"web_url"`
			Labels    []string  `json:"labels"`
			DueDate   *string   `json:"due_date"`
			State     string    `json:"state"`
			UpdatedAt time.Time `json:"updated_at"`
			Assignee  *struct {
				ID int64 `json:"id"`
			} `json:"assignee"`
		}
		decodeErr := decodeJSON(response.Body, &rows)
		page = response.Header.Get("X-Next-Page")
		response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode GitLab issues: %w", decodeErr)
		}
		for _, row := range rows {
			issue := appsync.GitLabIssue{
				IssueIID: row.IID, GitLabIssueID: row.ID, Title: row.Title,
				WebURL: row.WebURL, Labels: row.Labels, State: row.State, UpdatedAt: row.UpdatedAt,
			}
			if row.DueDate != nil {
				issue.DueDate = *row.DueDate
			}
			if row.Assignee != nil {
				id := row.Assignee.ID
				issue.AssigneeGitLabUserID = &id
			}
			result = append(result, issue)
		}
	}
	return result, nil
}

func (c *Client) ApplyIssue(ctx context.Context, mutation appsync.IssueMutation) (appsync.GitLabIssue, error) {
	payload := map[string]any{
		"title":    mutation.Title,
		"labels":   mutation.Labels,
		"due_date": mutation.DueDate,
	}
	method := http.MethodPut
	requestURL := c.projectEndpoint("/issues/") + strconv.FormatInt(mutation.IssueIID, 10)
	if mutation.Create {
		method = http.MethodPost
		requestURL = c.projectEndpoint("/issues")
		if mutation.AssigneeGitLabUserID != nil {
			payload["assignee_id"] = *mutation.AssigneeGitLabUserID
		}
	} else {
		assigneeIDs := []int64{}
		if mutation.AssigneeGitLabUserID != nil {
			assigneeIDs = append(assigneeIDs, *mutation.AssigneeGitLabUserID)
		}
		payload["assignee_ids"] = assigneeIDs
		if mutation.Closed {
			payload["state_event"] = "close"
		} else {
			payload["state_event"] = "reopen"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return appsync.GitLabIssue{}, fmt.Errorf("encode GitLab issue mutation: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, strings.NewReader(string(body)))
	if err != nil {
		return appsync.GitLabIssue{}, fmt.Errorf("create GitLab issue request: %w", err)
	}
	request.Header.Set("PRIVATE-TOKEN", c.config.AccessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return appsync.GitLabIssue{}, identity.ErrGitLabUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode >= 500 {
		return appsync.GitLabIssue{}, identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return appsync.GitLabIssue{}, &httpStatusError{status: response.StatusCode}
	}
	var wire issueWire
	if err := decodeJSON(response.Body, &wire); err != nil {
		return appsync.GitLabIssue{}, fmt.Errorf("decode GitLab issue mutation: %w", err)
	}
	return mapIssueWire(wire), nil
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
		"scope":                 {"read_api"},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return c.endpoint("/oauth/authorize") + "?" + values.Encode()
}

func (c *Client) ExchangeIdentity(ctx context.Context, code, verifier string) (appoauth.GitLabIdentity, error) {
	accessToken, err := c.exchangeToken(ctx, code, verifier)
	if err != nil {
		return appoauth.GitLabIdentity{}, err
	}
	var user gitLabUser
	if err := c.get(ctx, c.endpoint("/api/v4/user"), accessToken, &user); err != nil {
		return appoauth.GitLabIdentity{}, err
	}
	var member gitLabMember
	memberURL := c.endpoint("/api/v4/projects/") + url.PathEscape(c.config.ProjectPath) + "/members/all/" + strconv.FormatInt(user.ID, 10)
	if err := c.get(ctx, memberURL, accessToken, &member); err != nil {
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
		AccessLevel: member.AccessLevel, State: member.State,
	}, nil
}

func (c *Client) exchangeToken(ctx context.Context, code, verifier string) (string, error) {
	values := url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.config.RedirectURI},
		"code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/oauth/token"), strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("create GitLab token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: exchange OAuth token", identity.ErrGitLabUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode >= 500 {
		return "", identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", &httpStatusError{status: response.StatusCode}
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := decodeJSON(response.Body, &token); err != nil {
		return "", fmt.Errorf("decode GitLab token: %w", err)
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("GitLab token response omitted access_token")
	}
	return token.AccessToken, nil
}

func (c *Client) get(ctx context.Context, requestURL, accessToken string, target any) error {
	response, err := c.do(ctx, http.MethodGet, requestURL, nil, "", accessToken)
	if err != nil {
		return err
	}
	defer response.Body.Close()
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
	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("create GitLab request: %w", err)
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
		response.Body.Close()
		return nil, identity.ErrGitLabUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
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

type issueWire struct {
	ID        int64     `json:"id"`
	IID       int64     `json:"iid"`
	Title     string    `json:"title"`
	WebURL    string    `json:"web_url"`
	Labels    []string  `json:"labels"`
	DueDate   *string   `json:"due_date"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
	Assignee  *struct {
		ID int64 `json:"id"`
	} `json:"assignee"`
}

func mapIssueWire(row issueWire) appsync.GitLabIssue {
	issue := appsync.GitLabIssue{
		IssueIID: row.IID, GitLabIssueID: row.ID, Title: row.Title,
		WebURL: row.WebURL, Labels: row.Labels, State: row.State, UpdatedAt: row.UpdatedAt,
	}
	if row.DueDate != nil {
		issue.DueDate = *row.DueDate
	}
	if row.Assignee != nil {
		id := row.Assignee.ID
		issue.AssigneeGitLabUserID = &id
	}
	return issue
}
