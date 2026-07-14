package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	appauth "example.com/project-template/internal/controller/application/auth"
	apptask "example.com/project-template/internal/controller/application/task"
	appworkspace "example.com/project-template/internal/controller/application/workspace"
	"example.com/project-template/internal/controller/config"
	"example.com/project-template/internal/controller/infrastructure/postgres"
	pgauth "example.com/project-template/internal/controller/infrastructure/postgres/auth"
	pgtask "example.com/project-template/internal/controller/infrastructure/postgres/task"
	pgworkspace "example.com/project-template/internal/controller/infrastructure/postgres/workspace"
	"example.com/project-template/internal/controller/infrastructure/security"
	"example.com/project-template/internal/controller/logger"
	httpserver "example.com/project-template/internal/controller/transport/http"
	"example.com/project-template/internal/platform/observability"
)

type Application struct {
	Config  config.Config
	Log     *zap.Logger
	Server  *http.Server
	DB      *pgxpool.Pool
	Tracing *observability.Tracing
}

func New(ctx context.Context) (*Application, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	log, err := logger.New(cfg.Env, cfg.ServiceName, cfg.Version, cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	tracing, err := observability.NewTracing(ctx, cfg.ServiceName, cfg.Version, cfg.Observability.OTLPTracesEndpoint)
	if err != nil {
		_ = log.Sync()
		return nil, err
	}
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = tracing.Shutdown(context.Background())
		_ = log.Sync()
		return nil, err
	}

	tx := postgres.NewTransactor(pool)
	authRepo := pgauth.New(pool)
	workspaceRepo := pgworkspace.New(pool)
	taskRepo := pgtask.New(pool)
	tracer := otel.Tracer(cfg.ServiceName)
	authService := appauth.NewService(authRepo, tx, security.NewPasswordHasher(0), security.NewTokens(cfg.Session.HashKey), appauth.Config{IdleTTL: cfg.Session.IdleTTL, AbsoluteTTL: cfg.Session.AbsoluteTTL, TouchAfter: cfg.Session.TouchAfter}, tracer)
	workspaceService := appworkspace.NewService(workspaceRepo, authRepo, tx, tracer)
	taskService := apptask.NewService(taskRepo, workspaceRepo, tracer)
	metrics := observability.NewMetrics()
	router := httpserver.NewRouter(httpserver.Dependencies{
		Log: log, Auth: authService, Workspaces: workspaceService, Tasks: taskService,
		Cookie:         httpserver.CookieConfig{Name: cfg.Session.CookieName, Secure: cfg.Session.CookieSecure, TTL: cfg.Session.AbsoluteTTL},
		AllowedOrigins: cfg.HTTP.AllowedOrigins, RequestTimeout: cfg.HTTP.RequestTimeout,
		Readiness: func(ctx context.Context) error { return postgres.Ready(ctx, pool) },
		Metrics:   metrics, WebDir: cfg.HTTP.WebDir,
		APIName: "Project Template API", APIVersion: cfg.Version,
	})
	return &Application{Config: cfg, Log: log, Server: httpserver.NewServer(httpserver.ServerConfig{Addr: cfg.HTTP.Addr}, router), DB: pool, Tracing: tracing}, nil
}

func (a *Application) Run(ctx context.Context) error {
	serveErrors := make(chan error, 1)
	go func() {
		a.Log.Info("http_server_started", zap.String("address", a.Server.Addr))
		serveErrors <- a.Server.ListenAndServe()
	}()

	var runErr error
	select {
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("serve HTTP: %w", err)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.Config.ShutdownTimeout)
	defer cancel()
	shutdownErr := a.Server.Shutdown(shutdownCtx)
	traceErr := a.Tracing.Shutdown(shutdownCtx)
	a.DB.Close()
	if syncErr := a.Log.Sync(); syncErr != nil && !errors.Is(syncErr, http.ErrServerClosed) {
		a.Log.Debug("logger_sync_failed", zap.Error(syncErr))
	}
	return errors.Join(runErr, shutdownErr, traceErr)
}

func ShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
