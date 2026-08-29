package oauth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
	credential        identity.OAuthCredential
	session           identity.Session
	revocations       []identity.OAuthTokenRevocation
	activeSessions    bool
	retryCode         string
	upsertInTx        bool
	createSessionInTx bool
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
func (f *repoFake) UpsertOAuthCredential(_ context.Context, credential identity.OAuthCredential) error {
	f.credential = credential
	return nil
}
func (f *repoFake) ReplaceOAuthCredential(_ context.Context, credential identity.OAuthCredential, now time.Time) error {
	if f.credential.UserID != "" {
		f.revocations = append(f.revocations, identity.OAuthTokenRevocation{
			ID: uuid.NewString(), UserID: f.credential.UserID,
			AccessTokenCiphertext:  f.credential.AccessTokenCiphertext,
			RefreshTokenCiphertext: f.credential.RefreshTokenCiphertext,
			AvailableAt:            now, CreatedAt: now, UpdatedAt: now,
		})
	}
	f.credential = credential
	return nil
}
func (f *repoFake) OAuthCredential(context.Context, string) (identity.OAuthCredential, error) {
	if f.credential.UserID == "" {
		return identity.OAuthCredential{}, identity.ErrOAuthCredentialNotFound
	}
	return f.credential, nil
}
func (f *repoFake) OAuthCredentialForUpdate(ctx context.Context, userID string) (identity.OAuthCredential, error) {
	return f.OAuthCredential(ctx, userID)
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
	return f.session, nil
}
func (f *repoFake) TouchSession(_ context.Context, _ string, session identity.Session) error {
	f.touched = session
	return nil
}
func (f *repoFake) DeleteSessionByTokenHash(context.Context, []byte) (string, bool, error) {
	if f.session.ID == "" {
		return "", false, nil
	}
	userID := f.session.UserID
	f.session = identity.Session{}
	return userID, true, nil
}
func (*repoFake) DeleteExpiredSession(context.Context, string) error { return nil }
func (f *repoFake) HasActiveSessions(context.Context, string, time.Time) (bool, error) {
	return f.activeSessions, nil
}
func (f *repoFake) QueueAndDeleteOAuthCredential(_ context.Context, userID string, now time.Time) error {
	if f.credential.UserID == userID {
		f.revocations = append(f.revocations, identity.OAuthTokenRevocation{
			ID: uuid.NewString(), UserID: userID,
			AccessTokenCiphertext:  f.credential.AccessTokenCiphertext,
			RefreshTokenCiphertext: f.credential.RefreshTokenCiphertext,
			AvailableAt:            now, CreatedAt: now, UpdatedAt: now,
		})
		f.credential = identity.OAuthCredential{}
	}
	return nil
}
func (*repoFake) QueueOrphanedOAuthCredentials(context.Context, time.Time) error { return nil }
func (f *repoFake) ClaimOAuthRevocation(_ context.Context, now, leaseUntil time.Time) (identity.OAuthTokenRevocation, error) {
	if len(f.revocations) == 0 || f.revocations[0].AvailableAt.After(now) {
		return identity.OAuthTokenRevocation{}, identity.ErrOAuthRevocationNotFound
	}
	f.revocations[0].Attempts++
	f.revocations[0].AvailableAt = leaseUntil
	return f.revocations[0], nil
}
func (f *repoFake) CompleteOAuthRevocation(_ context.Context, id string) error {
	if len(f.revocations) > 0 && f.revocations[0].ID == id {
		f.revocations = f.revocations[1:]
	}
	return nil
}
func (f *repoFake) RetryOAuthRevocation(_ context.Context, id, code string, availableAt, now time.Time) error {
	if len(f.revocations) > 0 && f.revocations[0].ID == id {
		f.revocations[0].AvailableAt = availableAt
		f.revocations[0].UpdatedAt = now
	}
	f.retryCode = code
	return nil
}
func (f *repoFake) OAuthRevocationQueueStats(_ context.Context, now time.Time) (int64, float64, error) {
	if len(f.revocations) == 0 {
		return 0, 0, nil
	}
	return int64(len(f.revocations)), now.Sub(f.revocations[0].CreatedAt).Seconds(), nil
}

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
func (*tokensFake) Derive(purpose, value string) string { return "derived:" + purpose + ":" + value }
func (f *tokensFake) MatchesDerived(raw, purpose, value string) bool {
	return raw == f.Derive(purpose, value)
}

type cipherFake struct{}

func (cipherFake) Seal(purpose, subject, value string) ([]byte, error) {
	return []byte("sealed:" + purpose + ":" + subject + ":" + value), nil
}
func (cipherFake) Open(purpose, subject string, value []byte) (string, bool, error) {
	prefix := "sealed:" + purpose + ":" + subject + ":"
	if !strings.HasPrefix(string(value), prefix) {
		return "", false, errors.New("ciphertext context mismatch")
	}
	return strings.TrimPrefix(string(value), prefix), true, nil
}

type gitLabFake struct {
	identity  GitLabIdentity
	err       error
	revokeErr error
	verifier  string
	revoked   []string
}

func (*gitLabFake) AuthorizationURL(state, challenge string) string {
	return "https://gitlab.com/oauth/authorize?state=" + state + "&code_challenge=" + challenge
}
func (f *gitLabFake) ExchangeIdentity(_ context.Context, _, verifier string) (GitLabIdentity, error) {
	f.verifier = verifier
	return f.identity, f.err
}
func (f *gitLabFake) RefreshToken(context.Context, string) (OAuthTokens, error) {
	return OAuthTokens{AccessToken: "refreshed-access", RefreshToken: "refreshed-refresh", ExpiresAt: time.Unix(30_000, 0)}, f.err
}
func (f *gitLabFake) RevokeToken(_ context.Context, token string) error {
	if token != "" {
		f.revoked = append(f.revoked, token)
	}
	return f.revokeErr
}

func newService(repo *repoFake, tx *txFake, tokens *tokensFake, gitlab *gitLabFake) *Service {
	service := NewService(repo, tx, tokens, cipherFake{}, cipherFake{}, gitlab, Config{
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

func TestCSRFTokensAreStablePerSessionAndAcceptLegacyTokens(t *testing.T) {
	t.Parallel()
	repo, tokens := &repoFake{session: identity.Session{
		ID: "session-id", UserID: "10000000-0000-0000-0000-000000000001",
		TokenHash: []byte("digest:session"), ExpiresAt: time.Unix(20_000, 0),
	}}, &tokensFake{}
	service := newService(repo, &txFake{}, tokens, &gitLabFake{})
	claims := identity.SessionClaims{SessionID: repo.session.ID, UserID: repo.session.UserID}

	first, err := service.IssueCSRF(context.Background(), claims)
	if err != nil {
		t.Fatalf("IssueCSRF() error = %v", err)
	}
	second, err := service.IssueCSRF(context.Background(), claims)
	if err != nil || first != second {
		t.Fatalf("IssueCSRF() tokens = %q, %q, error = %v", first, second, err)
	}
	if _, err := service.VerifyCSRFToken(context.Background(), "session", first); err != nil {
		t.Fatalf("VerifyCSRFToken(derived) error = %v", err)
	}

	repo.session.CSRFTokenHash = tokens.Digest("legacy-csrf")
	if _, err := service.VerifyCSRFToken(context.Background(), "session", "legacy-csrf"); err != nil {
		t.Fatalf("VerifyCSRFToken(legacy) error = %v", err)
	}
	if _, err := service.VerifyCSRFToken(context.Background(), "session", tokens.Derive("csrf", "another-session")); err == nil {
		t.Fatal("VerifyCSRFToken() accepted another session's token")
	}
}

func TestStartStoresHashedStateAndPKCEVerifier(t *testing.T) {
	t.Parallel()
	repo, tokens, gitlab := &repoFake{}, &tokensFake{}, &gitLabFake{}
	result, err := newService(repo, &txFake{}, tokens, gitlab).Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !strings.HasPrefix(result.AuthorizationURL, "https://gitlab.com/oauth/authorize") || result.StateToken == "" ||
		len(repo.state.StateHash) == 0 || !strings.HasPrefix(string(repo.state.VerifierCiphertext), "sealed:") {
		t.Fatalf("Start() = %#v, state = %#v", result, repo.state)
	}
}

func TestCompleteConsumesStateAndCreatesSessionTransaction(t *testing.T) {
	t.Parallel()
	repo, tx, tokens := &repoFake{}, &txFake{}, &tokensFake{}
	gitlab := &gitLabFake{identity: GitLabIdentity{
		GitLabUserID: 123, Username: "yorukot", DisplayName: "Yorukot",
		ProfileURL: "https://gitlab.com/yorukot", AccessLevel: 40, State: "active",
		Tokens: OAuthTokens{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Unix(20_000, 0)},
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
	if !strings.HasSuffix(string(repo.credential.AccessTokenCiphertext), ":access") || repo.credential.UserID != result.User.ID {
		t.Fatalf("credential = %#v", repo.credential)
	}
	accessToken, err := service.AccessToken(context.Background(), result.User.ID)
	if err != nil || accessToken != "access" {
		t.Fatalf("AccessToken() = %q, %v", accessToken, err)
	}
	_, err = service.Complete(context.Background(), CompleteInput{Code: "code", State: "state"})
	assertAppError(t, err, apperror.KindUnauthorized, "AUTH_OAUTH_FAILED")
}

func TestAccessTokenRefreshesAndRotatesEncryptedCredential(t *testing.T) {
	t.Parallel()
	repo := &repoFake{credential: identity.OAuthCredential{
		UserID:                 "10000000-0000-0000-0000-000000000001",
		AccessTokenCiphertext:  []byte("sealed:" + accessTokenPurpose + ":10000000-0000-0000-0000-000000000001:expired-access"),
		RefreshTokenCiphertext: []byte("sealed:" + refreshTokenPurpose + ":10000000-0000-0000-0000-000000000001:old-refresh"),
		ExpiresAt:              time.Unix(10_000, 0), UpdatedAt: time.Unix(9_000, 0),
	}}
	service := newService(repo, &txFake{}, &tokensFake{}, &gitLabFake{})

	accessToken, err := service.AccessToken(context.Background(), repo.credential.UserID)
	if err != nil || accessToken != "refreshed-access" {
		t.Fatalf("AccessToken() = %q, %v", accessToken, err)
	}
	if !strings.HasSuffix(string(repo.credential.RefreshTokenCiphertext), ":refreshed-refresh") || !repo.credential.ExpiresAt.Equal(time.Unix(30_000, 0)) {
		t.Fatalf("refreshed credential = %#v", repo.credential)
	}
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

func TestCompleteRejectsRoleBelowPlannerAndRevokesIssuedTokens(t *testing.T) {
	t.Parallel()
	repo, tokens := &repoFake{}, &tokensFake{}
	gitlab := &gitLabFake{identity: GitLabIdentity{
		GitLabUserID: 123, Username: "guest", State: "active", AccessLevel: identity.PlannerAccessLevel - 1,
		Tokens: OAuthTokens{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Unix(20_000, 0)},
	}}
	service := newService(repo, &txFake{}, tokens, gitlab)
	if _, err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err := service.Complete(context.Background(), CompleteInput{Code: "code", State: "state"})
	assertAppError(t, err, apperror.KindForbidden, "FORBIDDEN")
	if !slices.Equal(gitlab.revoked, []string{"access", "refresh"}) {
		t.Fatalf("revoked tokens = %v", gitlab.revoked)
	}
}

func TestLogoutQueuesCredentialOnlyAfterLastSession(t *testing.T) {
	t.Parallel()
	userID := "10000000-0000-0000-0000-000000000001"
	newRepo := func(active bool) *repoFake {
		return &repoFake{
			session: identity.Session{ID: "session", UserID: userID},
			credential: identity.OAuthCredential{
				UserID: userID, AccessTokenCiphertext: []byte("access"), RefreshTokenCiphertext: []byte("refresh"),
			},
			activeSessions: active,
		}
	}
	remaining := newRepo(true)
	if err := newService(remaining, &txFake{}, &tokensFake{}, &gitLabFake{}).Logout(context.Background(), "session"); err != nil {
		t.Fatal(err)
	}
	if remaining.credential.UserID == "" || len(remaining.revocations) != 0 {
		t.Fatalf("credential was queued with another active session: %#v", remaining)
	}

	last := newRepo(false)
	if err := newService(last, &txFake{}, &tokensFake{}, &gitLabFake{}).Logout(context.Background(), "session"); err != nil {
		t.Fatal(err)
	}
	if last.credential.UserID != "" || len(last.revocations) != 1 {
		t.Fatalf("last-session logout did not queue credential: %#v", last)
	}
}

func TestDurableRevocationWorkerRevokesBothTokens(t *testing.T) {
	t.Parallel()
	userID := "10000000-0000-0000-0000-000000000001"
	access, _ := (cipherFake{}).Seal(accessTokenPurpose, userID, "access")
	refresh, _ := (cipherFake{}).Seal(refreshTokenPurpose, userID, "refresh")
	repo := &repoFake{revocations: []identity.OAuthTokenRevocation{{
		ID: uuid.NewString(), UserID: userID,
		AccessTokenCiphertext: access, RefreshTokenCiphertext: refresh,
		AvailableAt: time.Unix(9_000, 0), CreatedAt: time.Unix(9_000, 0),
	}}}
	gitlab := &gitLabFake{}
	processed, err := newService(repo, &txFake{}, &tokensFake{}, gitlab).processOAuthRevocation(context.Background())
	if err != nil || !processed {
		t.Fatalf("processOAuthRevocation() = %t, %v", processed, err)
	}
	if len(repo.revocations) != 0 || !slices.Equal(gitlab.revoked, []string{"access", "refresh"}) {
		t.Fatalf("revocation queue = %#v, revoked = %v", repo.revocations, gitlab.revoked)
	}
}

func TestDurableRevocationWorkerRevokesDecryptableTokenBeforeRetry(t *testing.T) {
	t.Parallel()
	userID := "10000000-0000-0000-0000-000000000001"
	access, _ := (cipherFake{}).Seal(accessTokenPurpose, userID, "access")
	repo := &repoFake{revocations: []identity.OAuthTokenRevocation{{
		ID: uuid.NewString(), UserID: userID,
		AccessTokenCiphertext: access, RefreshTokenCiphertext: []byte("corrupt"),
		AvailableAt: time.Unix(9_000, 0), CreatedAt: time.Unix(9_000, 0),
	}}}
	gitlab := &gitLabFake{}
	processed, err := newService(repo, &txFake{}, &tokensFake{}, gitlab).processOAuthRevocation(context.Background())
	if err != nil || !processed {
		t.Fatalf("processOAuthRevocation() = %t, %v", processed, err)
	}
	if len(repo.revocations) != 1 || repo.retryCode != "decrypt_failed" || !slices.Equal(gitlab.revoked, []string{"access"}) {
		t.Fatalf("revocation queue = %#v, retry = %q, revoked = %v", repo.revocations, repo.retryCode, gitlab.revoked)
	}
}

func assertAppError(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("error = %#v, want kind %s code %s", err, kind, code)
	}
}
