package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/identity"
)

type Config struct {
	IdleTTL     time.Duration
	AbsoluteTTL time.Duration
	TouchAfter  time.Duration
}

type Service struct {
	repo   Repository
	tx     Transactor
	hasher PasswordHasher
	tokens Tokens
	config Config
	now    func() time.Time
	tracer trace.Tracer
}

func NewService(repo Repository, tx Transactor, hasher PasswordHasher, tokens Tokens, cfg Config, tracer trace.Tracer) *Service {
	return &Service{repo: repo, tx: tx, hasher: hasher, tokens: tokens, config: cfg, now: time.Now, tracer: tracer}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (Authenticated, error) {
	ctx, span := s.tracer.Start(ctx, "auth.register")
	defer span.End()

	email := normalizeEmail(input.Email)
	displayName := strings.TrimSpace(input.DisplayName)
	fields := validateCredentials(email, input.Password, displayName, true)
	if len(fields) > 0 {
		return Authenticated{}, apperror.Invalid("VALIDATION_FAILED", "registration input is invalid", fields...)
	}
	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		span.RecordError(err)
		return Authenticated{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	user := identity.User{ID: uuid.NewString(), Email: email, PasswordHash: passwordHash, DisplayName: displayName, CreatedAt: now, UpdatedAt: now}
	raw, digest, err := s.tokens.New()
	if err != nil {
		span.RecordError(err)
		return Authenticated{}, fmt.Errorf("issue session token: %w", err)
	}
	session := identity.Session{
		ID: uuid.NewString(), UserID: user.ID, TokenHash: digest,
		IdleExpiresAt: now.Add(s.config.IdleTTL), AbsoluteExpiresAt: now.Add(s.config.AbsoluteTTL),
		CreatedAt: now, LastSeenAt: now,
	}

	err = s.tx.WithinTx(ctx, func(txCtx context.Context) error {
		var createErr error
		user, createErr = s.repo.CreateUser(txCtx, user)
		if createErr != nil {
			return createErr
		}
		_, createErr = s.repo.CreateSession(txCtx, session)
		return createErr
	})
	if errors.Is(err, identity.ErrEmailInUse) {
		return Authenticated{}, apperror.Conflict("EMAIL_ALREADY_EXISTS", "email is already registered")
	}
	if err != nil {
		span.RecordError(err)
		return Authenticated{}, fmt.Errorf("register user: %w", err)
	}
	return Authenticated{User: user, SessionToken: raw}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Authenticated, error) {
	ctx, span := s.tracer.Start(ctx, "auth.login")
	defer span.End()

	email := normalizeEmail(input.Email)
	if email == "" || input.Password == "" {
		return Authenticated{}, apperror.Invalid("VALIDATION_FAILED", "email and password are required")
	}
	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, identity.ErrUserNotFound) || (err == nil && s.hasher.Compare(user.PasswordHash, input.Password) != nil) {
		return Authenticated{}, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "email or password is incorrect")
	}
	if err != nil {
		span.RecordError(err)
		return Authenticated{}, fmt.Errorf("find login user: %w", err)
	}
	if err := s.hasher.Compare(user.PasswordHash, input.Password); err != nil {
		return Authenticated{}, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "email or password is incorrect")
	}
	raw, digest, err := s.tokens.New()
	if err != nil {
		span.RecordError(err)
		return Authenticated{}, fmt.Errorf("issue session token: %w", err)
	}
	now := s.now().UTC()
	session := identity.Session{ID: uuid.NewString(), UserID: user.ID, TokenHash: digest, IdleExpiresAt: now.Add(s.config.IdleTTL), AbsoluteExpiresAt: now.Add(s.config.AbsoluteTTL), CreatedAt: now, LastSeenAt: now}
	if _, err := s.repo.CreateSession(ctx, session); err != nil {
		span.RecordError(err)
		return Authenticated{}, fmt.Errorf("create login session: %w", err)
	}
	return Authenticated{User: user, SessionToken: raw}, nil
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
	if !now.Before(session.IdleExpiresAt) || !now.Before(session.AbsoluteExpiresAt) {
		_ = s.repo.DeleteExpiredSession(ctx, session.ID)
		return identity.SessionClaims{}, apperror.Unauthorized("AUTH_INVALID_SESSION", "session has expired")
	}
	if now.Sub(session.LastSeenAt) >= s.config.TouchAfter {
		nextIdle := now.Add(s.config.IdleTTL)
		if nextIdle.After(session.AbsoluteExpiresAt) {
			nextIdle = session.AbsoluteExpiresAt
		}
		session.LastSeenAt, session.IdleExpiresAt = now, nextIdle
		if err := s.repo.TouchSession(ctx, session.ID, session); err != nil {
			return identity.SessionClaims{}, fmt.Errorf("touch session: %w", err)
		}
	}
	return identity.SessionClaims{SessionID: session.ID, UserID: session.UserID, ExpiresAt: session.AbsoluteExpiresAt}, nil
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

func validateCredentials(email, password, displayName string, requireName bool) []apperror.Field {
	var fields []apperror.Field
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 {
		fields = append(fields, apperror.Field{Name: "email", Code: "INVALID_FORMAT", Message: "must be a valid email address"})
	}
	if len(password) < 12 || len(password) > 128 {
		fields = append(fields, apperror.Field{Name: "password", Code: "INVALID_LENGTH", Message: "must be between 12 and 128 characters"})
	}
	if requireName && (displayName == "" || len([]rune(displayName)) > 64) {
		fields = append(fields, apperror.Field{Name: "displayName", Code: "INVALID_LENGTH", Message: "must be between 1 and 64 characters"})
	}
	return fields
}
