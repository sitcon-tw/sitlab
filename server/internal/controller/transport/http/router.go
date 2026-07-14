package httpserver

import (
	"context"
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
	Workspaces     WorkspaceService
	Tasks          TaskService
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
		dep.APIName = "Project Template API"
	}
	if dep.APIVersion == "" {
		dep.APIVersion = "0.1.0"
	}
	h := handler{auth: dep.Auth, workspaces: dep.Workspaces, tasks: dep.Tasks, cookie: dep.Cookie}
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
		readiness := func(w http.ResponseWriter, r *http.Request) {
			if dep.Readiness != nil {
				if err := dep.Readiness(r.Context()); err != nil {
					writeError(w, r, apperror.Unavailable("service is not ready"))
					return
				}
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		}
		api.Get("/health/ready", readiness)
		api.Get("/healthz", readiness)
		api.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(openapi.Document())
		})
		api.Post("/auth/register", h.register)
		api.Post("/auth/login", h.login)

		api.Group(func(protected chi.Router) {
			protected.Use(requireAuth(dep.Auth, dep.Cookie.Name))
			protected.Use(requireCSRF(dep.Auth, dep.Cookie.Name, dep.AllowedOrigins))
			protected.Get("/auth/csrf", h.csrf)
			protected.Get("/auth/me", h.me)
			protected.Post("/auth/logout", h.logout)
			protected.Get("/workspaces", h.listWorkspaces)
			protected.Post("/workspaces", h.createWorkspace)
			protected.Get("/workspaces/{workspaceId}", h.getWorkspace)
			protected.Patch("/workspaces/{workspaceId}", h.updateWorkspace)
			protected.Delete("/workspaces/{workspaceId}", h.deleteWorkspace)
			protected.Route("/workspaces/{workspaceId}", func(workspace chi.Router) {
				workspace.Get("/members", h.listMembers)
				workspace.Post("/members", h.addMember)
				workspace.Patch("/members/{userId}", h.updateMember)
				workspace.Delete("/members/{userId}", h.removeMember)
				workspace.Get("/tasks", h.listTasks)
				workspace.Post("/tasks", h.createTask)
				workspace.Get("/tasks/{taskId}", h.getTask)
				workspace.Patch("/tasks/{taskId}", h.updateTask)
				workspace.Delete("/tasks/{taskId}", h.deleteTask)
			})
		})
		api.MethodNotAllowed(methodNotAllowed)
		api.NotFound(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, apperror.NotFound("route")) })
	})

	if strings.TrimSpace(dep.WebDir) != "" {
		router.Handle("/*", spaHandler(dep.WebDir))
	} else {
		router.NotFound(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, apperror.NotFound("route")) })
	}
	return router
}

func spaHandler(root string) http.Handler {
	root, _ = filepath.Abs(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate := filepath.Join(root, filepath.Clean("/"+r.URL.Path))
		if !strings.HasPrefix(candidate, root+string(os.PathSeparator)) && candidate != root {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			http.ServeFile(w, r, candidate)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}
