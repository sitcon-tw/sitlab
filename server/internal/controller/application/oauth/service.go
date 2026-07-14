package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/identity"
)

type Config struct {
	OAuthStateTTL time.Duration
	SessionTTL    time.Duration
}

type Service struct {
	repo   Repository
	tx     Transactor
	tokens Tokens
	cipher Cipher
	gitlab GitLab
	config Config
	now    func() time.Time
	tracer trace.Tracer
}

func NewService(repo Repository, tx Transactor, tokens Tokens, cipher Cipher, gitlab GitLab, cfg Config, tracer trace.Tracer) *Service {
	return &Service{repo: repo, tx: tx, tokens: tokens, cipher: cipher, gitlab: gitlab, config: cfg, now: time.Now, tracer: tracer}
}

func (s *Service) Start(ctx context.Context) (StartResult, error) {
	ctx, span := s.tracer.Start(ctx, "auth.gitlab.start")
	defer span.End()
	state, stateHash, err := s.tokens.New()
	if err != nil {
		return StartResult{}, technical(span, "create oauth state", err)
	}
	verifier, _, err := s.tokens.New()
	if err != nil {
		return StartResult{}, technical(span, "create PKCE verifier", err)
	}
	ciphertext, err := s.cipher.Seal(verifier)
	if err != nil {
		return StartResult{}, technical(span, "seal PKCE verifier", err)
	}
	now := s.now().UTC()
	if err := s.repo.StoreOAuthState(ctx, identity.OAuthState{
		StateHash: stateHash, VerifierCiphertext: ciphertext, ReturnPath: "/",
		ExpiresAt: now.Add(s.config.OAuthStateTTL), CreatedAt: now,
	}); err != nil {
		return StartResult{}, technical(span, "store oauth state", err)
	}
	challenge := sha256.Sum256([]byte(verifier))
	return StartResult{AuthorizationURL: s.gitlab.AuthorizationURL(state, base64.RawURLEncoding.EncodeToString(challenge[:]))}, nil
}

func (s *Service) Complete(ctx context.Context, input CompleteInput) (Authenticated, error) {
	ctx, span := s.tracer.Start(ctx, "auth.gitlab.complete")
	defer span.End()
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.State) == "" {
		return Authenticated{}, apperror.Malformed("GitLab callback is missing code or state")
	}
	state, err := s.repo.ConsumeOAuthState(ctx, s.tokens.Digest(input.State))
	if errors.Is(err, identity.ErrOAuthStateNotFound) {
		return Authenticated{}, apperror.Unauthorized("AUTH_OAUTH_FAILED", "OAuth state is invalid or already used")
	}
	if err != nil {
		return Authenticated{}, technical(span, "consume oauth state", err)
	}
	if !s.now().UTC().Before(state.ExpiresAt) {
		return Authenticated{}, apperror.Unauthorized("AUTH_OAUTH_FAILED", "OAuth state has expired")
	}
	verifier, err := s.cipher.Open(state.VerifierCiphertext)
	if err != nil {
		return Authenticated{}, technical(span, "open PKCE verifier", err)
	}
	gitLabIdentity, err := s.gitlab.ExchangeIdentity(ctx, input.Code, verifier)
	if errors.Is(err, identity.ErrProjectMemberRequired) {
		return Authenticated{}, apperror.Forbidden("FORBIDDEN", "an active SITCON 2027 project membership is required")
	}
	if errors.Is(err, identity.ErrGitLabUnavailable) {
		return Authenticated{}, apperror.Unavailable("GitLab is temporarily unavailable")
	}
	if err != nil {
		return Authenticated{}, apperror.Unauthorized("AUTH_OAUTH_FAILED", "GitLab authorization failed")
	}
	if gitLabIdentity.GitLabUserID <= 0 || strings.TrimSpace(gitLabIdentity.Username) == "" || gitLabIdentity.State != "active" || gitLabIdentity.AccessLevel <= 0 {
		return Authenticated{}, apperror.Forbidden("FORBIDDEN", "an active SITCON 2027 project membership is required")
	}

	now := s.now().UTC()
	user := identity.User{
		ID: uuid.NewString(), GitLabUserID: gitLabIdentity.GitLabUserID,
		Username: strings.TrimSpace(gitLabIdentity.Username), DisplayName: strings.TrimSpace(gitLabIdentity.DisplayName),
		AvatarURL: strings.TrimSpace(gitLabIdentity.AvatarURL), ProfileURL: strings.TrimSpace(gitLabIdentity.ProfileURL),
		AccessLevel: gitLabIdentity.AccessLevel, CreatedAt: now, UpdatedAt: now,
	}
	rawSession, sessionHash, err := s.tokens.New()
	if err != nil {
		return Authenticated{}, technical(span, "issue session token", err)
	}
	session := identity.Session{
		ID: uuid.NewString(), TokenHash: sessionHash,
		ExpiresAt: now.Add(s.config.SessionTTL), CreatedAt: now, LastUsedAt: now,
	}
	err = s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		var upsertErr error
		user, upsertErr = s.repo.UpsertUser(txCtx, user)
		if upsertErr != nil {
			return upsertErr
		}
		session.UserID = user.ID
		_, upsertErr = s.repo.CreateSession(txCtx, session)
		return upsertErr
	})
	if err != nil {
		return Authenticated{}, technical(span, "create GitLab session", err)
	}
	return Authenticated{User: user, SessionToken: rawSession, RedirectPath: state.ReturnPath}, nil
}

func (s *Service) VerifySession(ctx context.Context, raw string) (identity.SessionClaims, error) {
	if strings.TrimSpace(raw) == "" {
		return identity.SessionClaims{}, apperror.Unauthorized("AUTH_MISSING_SESSION", "authentication is required")
	}
	session, err := s.repo.GetSessionByTokenHash(ctx, s.tokens.Digest(raw))
	if errors.Is(err, identity.ErrSessionNotFound) {
		return identity.SessionClaims{}, apperror.Unauthorized("AUTH_INVALID_SESSION", "session is invalid")
	}
	if err != nil {
		return identity.SessionClaims{}, fmt.Errorf("verify session: %w", err)
	}
	now := s.now().UTC()
	if !now.Before(session.ExpiresAt) {
		_ = s.repo.DeleteExpiredSession(ctx, session.ID)
		return identity.SessionClaims{}, apperror.Unauthorized("AUTH_INVALID_SESSION", "session has expired")
	}
	session.LastUsedAt, session.ExpiresAt = now, now.Add(s.config.SessionTTL)
	if err := s.repo.TouchSession(ctx, session.ID, session); err != nil {
		return identity.SessionClaims{}, fmt.Errorf("renew session: %w", err)
	}
	return identity.SessionClaims{SessionID: session.ID, UserID: session.UserID, ExpiresAt: session.ExpiresAt}, nil
}

func (s *Service) IssueCSRF(ctx context.Context, claims identity.SessionClaims) (string, error) {
	raw, digest, err := s.tokens.New()
	if err != nil {
		return "", fmt.Errorf("issue csrf token: %w", err)
	}
	if err := s.repo.SetSessionCSRFHash(ctx, claims.SessionID, digest); err != nil {
		return "", fmt.Errorf("store csrf token: %w", err)
	}
	return raw, nil
}

func (s *Service) VerifyCSRFToken(ctx context.Context, rawSession, rawCSRF string) (identity.SessionClaims, error) {
	claims, err := s.VerifySession(ctx, rawSession)
	if err != nil {
		return identity.SessionClaims{}, err
	}
	session, err := s.repo.GetSessionByTokenHash(ctx, s.tokens.Digest(rawSession))
	if err != nil {
		return identity.SessionClaims{}, fmt.Errorf("load session csrf: %w", err)
	}
	if len(session.CSRFTokenHash) == 0 || !s.tokens.Matches(rawCSRF, session.CSRFTokenHash) {
		return identity.SessionClaims{}, apperror.Forbidden("AUTH_INVALID_CSRF", "csrf token is invalid")
	}
	return claims, nil
}

func (s *Service) Logout(ctx context.Context, rawSession string) error {
	if rawSession == "" {
		return nil
	}
	if err := s.repo.DeleteSessionByTokenHash(ctx, s.tokens.Digest(rawSession)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Service) Me(ctx context.Context, userID string) (identity.User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if errors.Is(err, identity.ErrUserNotFound) {
		return identity.User{}, apperror.NotFound("user")
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("get current user: %w", err)
	}
	return user, nil
}

func technical(span trace.Span, action string, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, action)
	return fmt.Errorf("%s: %w", action, err)
}
