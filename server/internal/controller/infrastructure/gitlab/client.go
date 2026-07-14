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

	appoauth "example.com/project-template/internal/controller/application/oauth"
	"example.com/project-template/internal/domain/identity"
)

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	ProjectPath  string
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
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create GitLab request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := c.http.Do(request)
	if err != nil {
		return identity.ErrGitLabUnavailable
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

func (c *Client) endpoint(path string) string {
	return strings.TrimRight(c.base.String(), "/") + path
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
