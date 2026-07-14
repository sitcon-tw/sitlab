package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/identity"
)

type authTxKey struct{}

type authTxFake struct{ calls int }

func (f *authTxFake) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(context.WithValue(ctx, authTxKey{}, true))
}

type authRepoFake struct {
	user              identity.User
	session           identity.Session
	createUserErr     error
	createUserInTx    bool
	createSessionInTx bool
	createSessions    int
	deletedExpired    bool
	csrfHash          []byte
}

func (f *authRepoFake) CreateUser(ctx context.Context, user identity.User) (identity.User, error) {
	f.createUserInTx, _ = ctx.Value(authTxKey{}).(bool)
	if f.createUserErr != nil {
		return identity.User{}, f.createUserErr
	}
	f.user = user
	return user, nil
}
func (f *authRepoFake) GetUserByEmail(context.Context, string) (identity.User, error) {
	if f.user.ID == "" {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.user, nil
}
func (f *authRepoFake) GetUserByID(context.Context, string) (identity.User, error) {
	return f.user, nil
}
func (f *authRepoFake) CreateSession(ctx context.Context, session identity.Session) (identity.Session, error) {
	f.createSessionInTx, _ = ctx.Value(authTxKey{}).(bool)
	f.createSessions++
	f.session = session
	return session, nil
}
func (f *authRepoFake) GetSessionByTokenHash(context.Context, []byte) (identity.Session, error) {
	if f.session.ID == "" {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	copy := f.session
	copy.CSRFTokenHash = f.csrfHash
	return copy, nil
}
func (f *authRepoFake) SetSessionCSRFHash(_ context.Context, _ string, digest []byte) error {
	f.csrfHash = digest
	return nil
}
func (*authRepoFake) TouchSession(context.Context, string, identity.Session) error { return nil }
func (*authRepoFake) DeleteSessionByTokenHash(context.Context, []byte) error       { return nil }
func (f *authRepoFake) DeleteExpiredSession(context.Context, string) error {
	f.deletedExpired = true
	return nil
}

type hasherFake struct{ compareErr error }

func (hasherFake) Hash(string) (string, error)    { return "password-hash", nil }
func (h hasherFake) Compare(string, string) error { return h.compareErr }

type tokensFake struct{ next int }

func (f *tokensFake) New() (string, []byte, error) {
	f.next++
	raw := "token-" + string(rune('0'+f.next))
	return raw, f.Digest(raw), nil
}
func (*tokensFake) Digest(raw string) []byte { return []byte("digest:" + raw) }
func (f *tokensFake) Matches(raw string, digest []byte) bool {
	return string(f.Digest(raw)) == string(digest)
}

func newAuthService(repo *authRepoFake, tx *authTxFake, hasher hasherFake, tokens *tokensFake) *Service {
	service := NewService(repo, tx, hasher, tokens, Config{IdleTTL: time.Hour, AbsoluteTTL: 24 * time.Hour, TouchAfter: 5 * time.Minute}, noop.NewTracerProvider().Tracer("test"))
	service.now = func() time.Time { return time.Unix(10_000, 0).UTC() }
	return service
}

func TestRegisterCreatesUserAndSessionInOneTransaction(t *testing.T) {
	repo, tx, tokens := &authRepoFake{}, &authTxFake{}, &tokensFake{}
	result, err := newAuthService(repo, tx, hasherFake{}, tokens).Register(context.Background(), RegisterInput{Email: "USER@example.com", Password: "a-secure-password", DisplayName: " User "})
	if err != nil {
		t.Fatal(err)
	}
	if tx.calls != 1 || !repo.createUserInTx || !repo.createSessionInTx {
		t.Fatalf("transaction invariant failed: calls=%d user=%v session=%v", tx.calls, repo.createUserInTx, repo.createSessionInTx)
	}
	if result.User.Email != "user@example.com" || result.User.DisplayName != "User" || result.SessionToken == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRegisterMapsEmailConflict(t *testing.T) {
	repo, tx, tokens := &authRepoFake{createUserErr: identity.ErrEmailInUse}, &authTxFake{}, &tokensFake{}
	_, err := newAuthService(repo, tx, hasherFake{}, tokens).Register(context.Background(), RegisterInput{Email: "user@example.com", Password: "a-secure-password", DisplayName: "User"})
	assertAppError(t, err, apperror.KindConflict, "EMAIL_ALREADY_EXISTS")
}

func TestLoginRejectsInvalidCredentialsWithoutCreatingSession(t *testing.T) {
	repo := &authRepoFake{user: identity.User{ID: "user-id", Email: "user@example.com", PasswordHash: "stored"}}
	_, err := newAuthService(repo, &authTxFake{}, hasherFake{compareErr: errors.New("mismatch")}, &tokensFake{}).Login(context.Background(), LoginInput{Email: "user@example.com", Password: "wrong"})
	assertAppError(t, err, apperror.KindUnauthorized, "AUTH_INVALID_CREDENTIALS")
	if repo.createSessions != 0 {
		t.Fatal("session created for invalid credentials")
	}
}

func TestVerifySessionExpiresAndDeletesStaleSession(t *testing.T) {
	repo := &authRepoFake{session: identity.Session{ID: "session-id", UserID: "user-id", IdleExpiresAt: time.Unix(9_999, 0), AbsoluteExpiresAt: time.Unix(20_000, 0)}}
	_, err := newAuthService(repo, &authTxFake{}, hasherFake{}, &tokensFake{}).VerifySession(context.Background(), "session")
	assertAppError(t, err, apperror.KindUnauthorized, "AUTH_INVALID_SESSION")
	if !repo.deletedExpired {
		t.Fatal("expired session was not removed")
	}
}

func TestCSRFTokenMustMatchSessionDigest(t *testing.T) {
	repo := &authRepoFake{session: identity.Session{ID: "session-id", UserID: "user-id", IdleExpiresAt: time.Unix(20_000, 0), AbsoluteExpiresAt: time.Unix(30_000, 0)}, csrfHash: []byte("digest:csrf")}
	service := newAuthService(repo, &authTxFake{}, hasherFake{}, &tokensFake{})
	if _, err := service.VerifyCSRFToken(context.Background(), "session", "csrf"); err != nil {
		t.Fatalf("valid csrf token rejected: %v", err)
	}
	_, err := service.VerifyCSRFToken(context.Background(), "session", "wrong")
	assertAppError(t, err, apperror.KindForbidden, "AUTH_INVALID_CSRF")
}

func assertAppError(t *testing.T, err error, kind apperror.Kind, code string) {
	t.Helper()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Kind != kind || appErr.Code != code {
		t.Fatalf("unexpected error: %#v", err)
	}
}
