package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/identity"
)

type txKey struct{}

type txFake struct{ calls int }

func (f *txFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(context.WithValue(ctx, txKey{}, true))
}

type repoFake struct {
	state             identity.OAuthState
	stateConsumed     bool
	user              identity.User
	session           identity.Session
	upsertInTx        bool
	createSessionInTx bool
	csrfHash          []byte
	touched           identity.Session
}

func (f *repoFake) StoreOAuthState(_ context.Context, state identity.OAuthState) error {
	f.state = state
	return nil
}
func (f *repoFake) ConsumeOAuthState(context.Context, []byte) (identity.OAuthState, error) {
	if f.stateConsumed || len(f.state.StateHash) == 0 {
		return identity.OAuthState{}, identity.ErrOAuthStateNotFound
	}
	f.stateConsumed = true
	return f.state, nil
}
func (f *repoFake) UpsertUser(ctx context.Context, user identity.User) (identity.User, error) {
	f.upsertInTx, _ = ctx.Value(txKey{}).(bool)
	if f.user.ID != "" {
		user.ID, user.CreatedAt = f.user.ID, f.user.CreatedAt
	}
	f.user = user
	return user, nil
}
func (f *repoFake) GetUserByID(context.Context, string) (identity.User, error) { return f.user, nil }
func (f *repoFake) CreateSession(ctx context.Context, session identity.Session) (identity.Session, error) {
	f.createSessionInTx, _ = ctx.Value(txKey{}).(bool)
	f.session = session
	return session, nil
}
func (f *repoFake) GetSessionByTokenHash(context.Context, []byte) (identity.Session, error) {
	if f.session.ID == "" {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	copy := f.session
	copy.CSRFTokenHash = f.csrfHash
	return copy, nil
}
func (f *repoFake) SetSessionCSRFHash(_ context.Context, _ string, hash []byte) error {
	f.csrfHash = hash
	return nil
}
func (f *repoFake) TouchSession(_ context.Context, _ string, session identity.Session) error {
	f.touched = session
	return nil
}
func (*repoFake) DeleteSessionByTokenHash(context.Context, []byte) error { return nil }
func (*repoFake) DeleteExpiredSession(context.Context, string) error     { return nil }

type tokensFake struct{ count int }

func (f *tokensFake) New() (string, []byte, error) {
	f.count++
	raw := strings.Repeat(string(rune('a'+f.count)), 43)
	return raw, f.Digest(raw), nil
}
func (*tokensFake) Digest(raw string) []byte { return []byte("digest:" + raw) }
func (f *tokensFake) Matches(raw string, hash []byte) bool {
	return string(f.Digest(raw)) == string(hash)
}

type cipherFake struct{}

func (cipherFake) Seal(value string) ([]byte, error) { return []byte("sealed:" + value), nil }
func (cipherFake) Open(value []byte) (string, error) {
	return strings.TrimPrefix(string(value), "sealed:"), nil
}

type gitLabFake struct {
	identity GitLabIdentity
	err      error
	verifier string
}

func (*gitLabFake) AuthorizationURL(state, challenge string) string {
	return "https://gitlab.com/oauth/authorize?state=" + state + "&code_challenge=" + challenge
}
func (f *gitLabFake) ExchangeIdentity(_ context.Context, _, verifier string) (GitLabIdentity, error) {
	f.verifier = verifier
	return f.identity, f.err
}

func newService(repo *repoFake, tx *txFake, tokens *tokensFake, gitlab *gitLabFake) *Service {
	service := NewService(repo, tx, tokens, cipherFake{}, gitlab, Config{
		OAuthStateTTL: 10 * time.Minute, SessionTTL: 14 * 24 * time.Hour,
	}, noop.NewTracerProvider().Tracer("test"))
	service.now = func() time.Time { return time.Unix(10_000, 0).UTC() }
	return service
}

func TestVerifySessionRenewsFourteenDaysFromEveryUse(t *testing.T) {
	t.Parallel()
	repo, tokens := &repoFake{
		session: identity.Session{
			ID: "session-id", UserID: "10000000-0000-0000-0000-000000000001",
			ExpiresAt: time.Unix(20_000, 0), LastUsedAt: time.Unix(9_000, 0),
		},
	}, &tokensFake{}
	service := newService(repo, &txFake{}, tokens, &gitLabFake{})
	claims, err := service.VerifySession(context.Background(), "session")
	if err != nil {
		t.Fatalf("VerifySession() error = %v", err)
	}
	wantExpiry := time.Unix(10_000, 0).UTC().Add(14 * 24 * time.Hour)
	if !repo.touched.ExpiresAt.Equal(wantExpiry) || !claims.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("renewed expiry = %s, claims = %s, want %s", repo.touched.ExpiresAt, claims.ExpiresAt, wantExpiry)
	}
}

func TestStartStoresHashedStateAndPKCEVerifier(t *testing.T) {
	t.Parallel()
	repo, tokens, gitlab := &repoFake{}, &tokensFake{}, &gitLabFake{}
	result, err := newService(repo, &txFake{}, tokens, gitlab).Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !strings.HasPrefix(result.AuthorizationURL, "https://gitlab.com/oauth/authorize") || len(repo.state.StateHash) == 0 || !strings.HasPrefix(string(repo.state.VerifierCiphertext), "sealed:") {
		t.Fatalf("Start() = %#v, state = %#v", result, repo.state)
	}
}

func TestCompleteConsumesStateAndCreatesSessionTransaction(t *testing.T) {
	t.Parallel()
	repo, tx, tokens := &repoFake{}, &txFake{}, &tokensFake{}
	gitlab := &gitLabFake{identity: GitLabIdentity{
		GitLabUserID: 123, Username: "yorukot", DisplayName: "Yorukot",
		ProfileURL: "https://gitlab.com/yorukot", AccessLevel: 40, State: "active",
	}}
	service := newService(repo, tx, tokens, gitlab)
	if _, err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := service.Complete(context.Background(), CompleteInput{Code: "code", State: "state"})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.User.GitLabUserID != 123 || result.SessionToken == "" || tx.calls != 1 || !repo.upsertInTx || !repo.createSessionInTx {
		t.Fatalf("Complete() = %#v, repo = %#v", result, repo)
	}
	if gitlab.verifier == "" {
		t.Fatal("PKCE verifier was not used")
	}
	_, err = service.Complete(context.Background(), CompleteInput{Code: "code", State: "state"})
	assertAppError(t, err, apperror.KindUnauthorized, "AUTH_OAUTH_FAILED")
}

func TestCompleteRejectsNonProjectMember(t *testing.T) {
	t.Parallel()
	repo, tokens := &repoFake{}, &tokensFake{}
	gitlab := &gitLabFake{err: identity.ErrProjectMemberRequired}
	service := newService(repo, &txFake{}, tokens, gitlab)
	if _, err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := service.Complete(context.Background(), CompleteInput{Code: "code", State: "state"})
	assertAppError(t, err, apperror.KindForbidden, "FORBIDDEN")
}

func assertAppError(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("error = %#v, want kind %s code %s", err, kind, code)
	}
}
