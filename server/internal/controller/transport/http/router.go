package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/controller/transport/http/openapi"
	"example.com/project-template/internal/platform/observability"
)

type Dependencies struct {
	Log            *zap.Logger
	Auth           AuthService
	Bootstrap      BootstrapService
	Directory      DirectoryService
	Board          BoardService
	Sync           SyncService
	Cookie         CookieConfig
	AllowedOrigins []string
	RequestTimeout time.Duration
	Readiness      func(context.Context) error
	Metrics        *observability.Metrics
	WebDir         string
	APIName        string
	APIVersion     string
}

func NewRouter(dep Dependencies) http.Handler {
	if dep.Log == nil {
		dep.Log = zap.NewNop()
	}
	if dep.RequestTimeout <= 0 {
		dep.RequestTimeout = 15 * time.Second
	}
	if dep.Metrics == nil {
		dep.Metrics = observability.NewMetrics()
	}
	if dep.APIName == "" {
		dep.APIName = "SITCON Board API"
	}
	if dep.APIVersion == "" {
		dep.APIVersion = "0.1.0"
	}
	h := handler{
		auth: dep.Auth, bootstrap: dep.Bootstrap, directory: dep.Directory,
		board: dep.Board, sync: dep.Sync, cookie: dep.Cookie,
	}
	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(securityHeaders)
	router.Use(recoverer(dep.Log))
	router.Use(chimiddleware.Timeout(dep.RequestTimeout))
	router.Use(otelhttp.NewMiddleware("http.server"))
	router.Use(dep.Metrics.Middleware)
	router.Use(requestLogger(dep.Log))
	router.Handle("/metrics", dep.Metrics.Handler())

	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"name": dep.APIName, "version": dep.APIVersion})
		})
		api.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		api.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
			if dep.Readiness != nil {
				if err := dep.Readiness(r.Context()); err != nil {
					writeError(w, r, apperror.Unavailable("service is not ready"))
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		api.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(openapi.Document())
		})
		api.Get("/auth/gitlab", h.startGitLabOAuth)
		api.Get("/auth/gitlab/callback", h.completeGitLabOAuth)

		api.Group(func(protected chi.Router) {
			protected.Use(requireAuth(dep.Auth, dep.Cookie))
			protected.Use(requireCSRF(dep.Auth, dep.Cookie.Name, dep.AllowedOrigins))
			protected.Get("/auth/csrf", h.csrf)
			protected.Get("/auth/me", h.me)
			protected.Post("/auth/logout", h.logout)
			protected.Get("/bootstrap", h.bootstrapState)
			protected.Get("/directory", h.directoryState)
			protected.Put("/me/preferences", h.updatePreferences)
			protected.Post("/cards", h.createCard)
			protected.Put("/cards/{issueIid}/details", h.updateCardDetails)
			protected.Put("/cards/{issueIid}/team", h.updateCardTeam)
			protected.Put("/cards/{issueIid}/assignee", h.updateCardAssignee)
			protected.Put("/cards/{issueIid}/start-date", h.updateCardStartDate)
			protected.Put("/cards/{issueIid}/due-date", h.updateCardDueDate)
			protected.Put("/cards/{issueIid}/position", h.moveCard)
			protected.Post("/operations/{operationId}/retry", h.retryOperation)
			protected.Post("/sync/refresh", h.refreshSnapshots)
		})
		api.MethodNotAllowed(methodNotAllowed)
		api.NotFound(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, apperror.NotFound("route")) })
	})

	if strings.TrimSpace(dep.WebDir) != "" {
		router.Handle("/*", spaHandler(dep.WebDir, dep.Auth, dep.Bootstrap, dep.Cookie))
	} else {
		router.NotFound(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, apperror.NotFound("route")) })
	}
	return router
}

func spaHandler(root string, auth AuthService, bootstrap BootstrapService, cookie CookieConfig) http.Handler {
	root, _ = filepath.Abs(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate := filepath.Join(root, filepath.Clean("/"+r.URL.Path))
		if !strings.HasPrefix(candidate, root+string(os.PathSeparator)) && candidate != root {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && filepath.Base(candidate) != "index.html" {
			http.ServeFile(w, r, candidate)
			return
		}
		indexPath := filepath.Join(root, "index.html")
		index, err := os.ReadFile(indexPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if sessionCookie, err := r.Cookie(cookie.Name); err == nil && sessionCookie.Value != "" {
			if claims, verifyErr := auth.VerifySession(r.Context(), sessionCookie.Value); verifyErr == nil {
				setRollingCookie(w, cookie, sessionCookie.Value, claims.ExpiresAt)
				if state, stateErr := bootstrap.Get(r.Context(), claims); stateErr == nil {
					payload, marshalErr := json.Marshal(mapBootstrap(state))
					if marshalErr == nil {
						script := append([]byte(`<script id="__SITCON_BOOTSTRAP__" type="application/json">`), payload...)
						script = append(script, []byte(`</script>`)...)
						index = bytes.Replace(index, []byte("</head>"), append(script, []byte("</head>")...), 1)
					}
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}
