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

	appboard "example.com/project-template/internal/controller/application/board"
	appbootstrap "example.com/project-template/internal/controller/application/bootstrap"
	appdirectory "example.com/project-template/internal/controller/application/directory"
	appoauth "example.com/project-template/internal/controller/application/oauth"
	appsync "example.com/project-template/internal/controller/application/sync"
	"example.com/project-template/internal/controller/config"
	"example.com/project-template/internal/controller/infrastructure/filedirectory"
	"example.com/project-template/internal/controller/infrastructure/gitlab"
	"example.com/project-template/internal/controller/infrastructure/postgres"
	pgoauth "example.com/project-template/internal/controller/infrastructure/postgres/oauth"
	pgsitcon "example.com/project-template/internal/controller/infrastructure/postgres/sitcon"
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
	Sync    *appsync.Service
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

	tokens := security.NewTokens(cfg.Session.HashKey)
	cipher, err := security.NewCipher(cfg.Session.CipherKey)
	if err != nil {
		pool.Close()
		_ = tracing.Shutdown(context.Background())
		_ = log.Sync()
		return nil, err
	}
	gitLabClient, err := gitlab.New(&http.Client{Timeout: cfg.HTTP.RequestTimeout}, gitlab.Config{
		BaseURL: cfg.GitLab.BaseURL, ClientID: cfg.GitLab.ClientID,
		ClientSecret: cfg.GitLab.ClientSecret, RedirectURI: cfg.GitLab.OAuthRedirectURL,
		ProjectPath: config.ProjectPath, AccessToken: cfg.GitLab.ProjectAccessToken,
	})
	if err != nil {
		pool.Close()
		_ = tracing.Shutdown(context.Background())
		_ = log.Sync()
		return nil, err
	}
	directoryClient, err := filedirectory.New(cfg.Directory.FilePath)
	if err != nil {
		pool.Close()
		_ = tracing.Shutdown(context.Background())
		_ = log.Sync()
		return nil, err
	}

	tracer := otel.Tracer(cfg.ServiceName)
	tx := postgres.NewTransactor(pool)
	oauthRepo := pgoauth.New(pool)
	store := pgsitcon.New(pool)
	oauthService := appoauth.NewService(oauthRepo, tx, tokens, cipher, gitLabClient, appoauth.Config{
		OAuthStateTTL: cfg.Session.OAuthStateTTL, SessionTTL: cfg.Session.TTL,
	}, tracer)
	directoryService := appdirectory.NewService(store, tracer)
	boardService := appboard.NewService(store, directoryService, tracer)
	syncService := appsync.NewService(gitLabClient, directoryClient, store, directoryLogger{log: log}, tracer)
	bootstrapService := appbootstrap.NewService(oauthService, directoryService, boardService, store)

	if syncErr := syncService.InitialSync(ctx); syncErr != nil {
		if readyErr := store.ReadySnapshots(ctx); readyErr != nil {
			pool.Close()
			_ = tracing.Shutdown(context.Background())
			_ = log.Sync()
			return nil, fmt.Errorf("initial source sync: %w", syncErr)
		}
		log.Warn("initial_source_sync_failed_using_snapshot", zap.Error(syncErr))
	}

	metrics := observability.NewMetrics()
	router := httpserver.NewRouter(httpserver.Dependencies{
		Log: log, Auth: oauthService, Bootstrap: bootstrapService,
		Directory: directoryService, Board: boardService, Sync: syncService,
		Cookie:         httpserver.CookieConfig{Name: cfg.Session.CookieName, Secure: cfg.Session.CookieSecure, TTL: cfg.Session.TTL},
		AllowedOrigins: cfg.HTTP.AllowedOrigins, RequestTimeout: cfg.HTTP.RequestTimeout,
		Readiness: func(ctx context.Context) error {
			return errors.Join(postgres.Ready(ctx, pool), store.ReadySnapshots(ctx))
		},
		Metrics: metrics, WebDir: cfg.HTTP.WebDir,
		APIName: "SITCON Board API", APIVersion: cfg.Version,
	})
	return &Application{
		Config: cfg, Log: log,
		Server: httpserver.NewServer(httpserver.ServerConfig{Addr: cfg.HTTP.Addr}, router),
		DB:     pool, Tracing: tracing, Sync: syncService,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	go a.Sync.Run(workerCtx, a.Config.Sync.DirectoryInterval, a.Config.Sync.BoardInterval)
	go a.Sync.RunOperations(workerCtx, a.Config.Sync.OperationInterval)

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
	cancelWorkers()

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

type directoryLogger struct{ log *zap.Logger }

func (l directoryLogger) DirectoryMemberMissing(teamKey, username string) {
	l.log.Warn("directory_member_missing_from_gitlab", zap.String("team_key", teamKey), zap.String("username", username))
}
