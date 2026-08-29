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

const (
	accessTokenPurpose  = "gitlab-access-token"
	refreshTokenPurpose = "gitlab-refresh-token"
	revocationLease     = 30 * time.Second
)

type Service struct {
	repo        Repository
	tx          Transactor
	tokens      Tokens
	stateCipher Cipher
	tokenCipher Cipher
	gitlab      GitLab
	config      Config
	now         func() time.Time
	tracer      trace.Tracer
	observer    Observer
}

func NewService(repo Repository, tx Transactor, tokens Tokens, stateCipher, tokenCipher Cipher, gitlab GitLab, cfg Config, tracer trace.Tracer) *Service {
	return &Service{
		repo: repo, tx: tx, tokens: tokens, stateCipher: stateCipher, tokenCipher: tokenCipher,
		gitlab: gitlab, config: cfg, now: time.Now, tracer: tracer,
	}
}

func (s *Service) SetObserver(observer Observer) { s.observer = observer }

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
	ciphertext, err := s.stateCipher.Seal("oauth-pkce-verifier", base64.RawURLEncoding.EncodeToString(stateHash), verifier)
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
	return StartResult{
		AuthorizationURL: s.gitlab.AuthorizationURL(state, base64.RawURLEncoding.EncodeToString(challenge[:])),
		StateToken:       state,
	}, nil
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
	verifier, _, err := s.stateCipher.Open(
		"oauth-pkce-verifier", base64.RawURLEncoding.EncodeToString(state.StateHash), state.VerifierCiphertext,
	)
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
	if gitLabIdentity.GitLabUserID <= 0 || strings.TrimSpace(gitLabIdentity.Username) == "" ||
		gitLabIdentity.State != "active" || gitLabIdentity.AccessLevel < identity.PlannerAccessLevel {
		s.revokeIssuedTokens(ctx, gitLabIdentity.Tokens)
		return Authenticated{}, apperror.Forbidden("FORBIDDEN", "an active SITCON 2027 project membership is required")
	}
	if gitLabIdentity.Tokens.AccessToken == "" || gitLabIdentity.Tokens.RefreshToken == "" || gitLabIdentity.Tokens.ExpiresAt.IsZero() {
		s.revokeIssuedTokens(ctx, gitLabIdentity.Tokens)
		return Authenticated{}, technical(span, "validate GitLab OAuth credential", errors.New("GitLab token response is incomplete"))
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
		credential, sealErr := s.sealCredential(user.ID, gitLabIdentity.Tokens, now)
		if sealErr != nil {
			return sealErr
		}
		if upsertErr = s.repo.ReplaceOAuthCredential(txCtx, credential, now); upsertErr != nil {
			return upsertErr
		}
		session.UserID = user.ID
		_, upsertErr = s.repo.CreateSession(txCtx, session)
		return upsertErr
	})
	if err != nil {
		revokeErr := s.revokeTokenPair(ctx, gitLabIdentity.Tokens.AccessToken, gitLabIdentity.Tokens.RefreshToken)
		return Authenticated{}, technical(span, "create GitLab session", errors.Join(err, revokeErr))
	}
	return Authenticated{User: user, SessionToken: rawSession, RedirectPath: state.ReturnPath}, nil
}

func (s *Service) AccessToken(ctx context.Context, userID string) (string, error) {
	var accessToken string
	refreshAttempted := false
	refreshed := false
	err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		credential, loadErr := s.repo.OAuthCredentialForUpdate(txCtx, userID)
		if loadErr != nil {
			return fmt.Errorf("load actor GitLab credential: %w", loadErr)
		}
		now := s.now().UTC()
		if now.Add(time.Minute).Before(credential.ExpiresAt) {
			var current bool
			accessToken, current, loadErr = s.tokenCipher.Open(accessTokenPurpose, userID, credential.AccessTokenCiphertext)
			if loadErr != nil {
				return fmt.Errorf("open actor GitLab access token: %w", loadErr)
			}
			if current {
				return nil
			}
			refreshToken, _, openErr := s.tokenCipher.Open(refreshTokenPurpose, userID, credential.RefreshTokenCiphertext)
			if openErr != nil {
				return fmt.Errorf("open actor GitLab refresh token for key rotation: %w", openErr)
			}
			rotated, sealErr := s.sealCredential(userID, OAuthTokens{
				AccessToken: accessToken, RefreshToken: refreshToken, ExpiresAt: credential.ExpiresAt,
			}, now)
			if sealErr != nil {
				return fmt.Errorf("rotate actor GitLab credential encryption: %w", sealErr)
			}
			return s.repo.UpsertOAuthCredential(txCtx, rotated)
		}
		refreshToken, _, openErr := s.tokenCipher.Open(refreshTokenPurpose, userID, credential.RefreshTokenCiphertext)
		if openErr != nil {
			return fmt.Errorf("open actor GitLab refresh token: %w", openErr)
		}
		refreshAttempted = true
		tokens, refreshErr := s.gitlab.RefreshToken(txCtx, refreshToken)
		if refreshErr != nil {
			return fmt.Errorf("refresh actor GitLab access token: %w", refreshErr)
		}
		rotated, sealErr := s.sealCredential(userID, tokens, now)
		if sealErr != nil {
			revokeErr := s.revokeTokenPair(txCtx, tokens.AccessToken, tokens.RefreshToken)
			return errors.Join(fmt.Errorf("seal refreshed actor GitLab credential: %w", sealErr), revokeErr)
		}
		if storeErr := s.repo.UpsertOAuthCredential(txCtx, rotated); storeErr != nil {
			revokeErr := s.revokeTokenPair(txCtx, tokens.AccessToken, tokens.RefreshToken)
			return errors.Join(fmt.Errorf("store refreshed actor GitLab credential: %w", storeErr), revokeErr)
		}
		accessToken = tokens.AccessToken
		refreshed = true
		return nil
	})
	if err != nil {
		if refreshAttempted {
			s.observeRefresh("failed")
		}
		return "", err
	}
	if refreshed {
		s.observeRefresh("succeeded")
	}
	return accessToken, nil
}

func (s *Service) sealCredential(userID string, tokens OAuthTokens, now time.Time) (identity.OAuthCredential, error) {
	accessCiphertext, err := s.tokenCipher.Seal(accessTokenPurpose, userID, tokens.AccessToken)
	if err != nil {
		return identity.OAuthCredential{}, err
	}
	refreshCiphertext, err := s.tokenCipher.Seal(refreshTokenPurpose, userID, tokens.RefreshToken)
	if err != nil {
		return identity.OAuthCredential{}, err
	}
	return identity.OAuthCredential{
		UserID: userID, AccessTokenCiphertext: accessCiphertext, RefreshTokenCiphertext: refreshCiphertext,
		ExpiresAt: tokens.ExpiresAt.UTC(), UpdatedAt: now,
	}, nil
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

func (s *Service) IssueCSRF(_ context.Context, claims identity.SessionClaims) (string, error) {
	return s.tokens.Derive("csrf", claims.SessionID), nil
}

func (s *Service) VerifyCSRFToken(ctx context.Context, rawSession, rawCSRF string) (identity.SessionClaims, error) {
	claims, err := s.VerifySession(ctx, rawSession)
	if err != nil {
		return identity.SessionClaims{}, err
	}
	if s.tokens.MatchesDerived(rawCSRF, "csrf", claims.SessionID) {
		return claims, nil
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
	now := s.now().UTC()
	return s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		userID, deleted, err := s.repo.DeleteSessionByTokenHash(txCtx, s.tokens.Digest(rawSession))
		if err != nil || !deleted {
			return err
		}
		active, err := s.repo.HasActiveSessions(txCtx, userID, now)
		if err != nil {
			return fmt.Errorf("check remaining auth sessions: %w", err)
		}
		if active {
			return nil
		}
		return s.repo.QueueAndDeleteOAuthCredential(txCtx, userID, now)
	})
}

func (s *Service) Maintain(ctx context.Context) error {
	now := s.now().UTC()
	if err := s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		return s.repo.QueueOrphanedOAuthCredentials(txCtx, now)
	}); err != nil {
		return fmt.Errorf("queue orphaned GitLab OAuth credentials: %w", err)
	}
	pending, oldest, err := s.repo.OAuthRevocationQueueStats(ctx, now)
	if err != nil {
		return fmt.Errorf("load GitLab OAuth revocation queue stats: %w", err)
	}
	if s.observer != nil {
		s.observer.SetOAuthRevocationQueue(pending, oldest)
	}
	return nil
}

func (s *Service) RunRevocations(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		for range 20 {
			processed, err := s.processOAuthRevocation(ctx)
			if err != nil {
				s.observeRevocation("worker_failed")
				break
			}
			if !processed {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) processOAuthRevocation(ctx context.Context) (bool, error) {
	now := s.now().UTC()
	item, err := s.repo.ClaimOAuthRevocation(ctx, now, now.Add(revocationLease))
	if errors.Is(err, identity.ErrOAuthRevocationNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	accessToken, _, accessErr := s.tokenCipher.Open(accessTokenPurpose, item.UserID, item.AccessTokenCiphertext)
	refreshToken, _, refreshErr := s.tokenCipher.Open(refreshTokenPurpose, item.UserID, item.RefreshTokenCiphertext)
	var revokeErr error
	if accessErr == nil {
		revokeErr = errors.Join(revokeErr, s.gitlab.RevokeToken(ctx, accessToken))
	}
	if refreshErr == nil {
		revokeErr = errors.Join(revokeErr, s.gitlab.RevokeToken(ctx, refreshToken))
	}
	if errors.Join(accessErr, refreshErr) != nil {
		s.observeRevocation("decrypt_failed")
		return true, s.retryOAuthRevocation(ctx, item, "decrypt_failed", now)
	}
	if revokeErr != nil {
		s.observeRevocation("retry")
		return true, s.retryOAuthRevocation(ctx, item, "gitlab_unavailable", now)
	}
	if err := s.repo.CompleteOAuthRevocation(ctx, item.ID); err != nil {
		return true, fmt.Errorf("complete GitLab OAuth token revocation: %w", err)
	}
	s.observeRevocation("succeeded")
	return true, nil
}

func (s *Service) retryOAuthRevocation(ctx context.Context, item identity.OAuthTokenRevocation, code string, now time.Time) error {
	shift := item.Attempts - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 12 {
		shift = 12
	}
	delay := time.Second * time.Duration(1<<shift)
	if delay > time.Hour {
		delay = time.Hour
	}
	if err := s.repo.RetryOAuthRevocation(ctx, item.ID, code, now.Add(delay), now); err != nil {
		return fmt.Errorf("retry GitLab OAuth token revocation: %w", err)
	}
	return nil
}

func (s *Service) revokeTokenPair(ctx context.Context, accessToken, refreshToken string) error {
	return errors.Join(s.gitlab.RevokeToken(ctx, accessToken), s.gitlab.RevokeToken(ctx, refreshToken))
}

func (s *Service) revokeIssuedTokens(ctx context.Context, tokens OAuthTokens) {
	if err := s.revokeTokenPair(ctx, tokens.AccessToken, tokens.RefreshToken); err != nil {
		s.observeRevocation("immediate_failed")
		return
	}
	s.observeRevocation("immediate_succeeded")
}

func (s *Service) observeRefresh(result string) {
	if s.observer != nil {
		s.observer.OAuthRefresh(result)
	}
}

func (s *Service) observeRevocation(result string) {
	if s.observer != nil {
		s.observer.OAuthRevocation(result)
	}
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
