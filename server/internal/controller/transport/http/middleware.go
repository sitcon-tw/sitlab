package httpserver

import (
	"context"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"example.com/project-template/internal/controller/application/apperror"
	"example.com/project-template/internal/domain/identity"
)

type authContextKey struct{}

func requireAuth(auth AuthService, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, r, apperror.Unauthorized("AUTH_MISSING_SESSION", "authentication is required"))
				return
			}
			claims, err := auth.VerifySession(r.Context(), cookie.Value)
			if err != nil {
				writeError(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authContextKey{}, claims)))
		})
	}
}

func requireCSRF(auth AuthService, cookieName string, allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			if !validOrigin(r, allowedOrigins) {
				writeError(w, r, apperror.Forbidden("AUTH_INVALID_CSRF", "request origin is not allowed"))
				return
			}
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, r, apperror.Unauthorized("AUTH_MISSING_SESSION", "authentication is required"))
				return
			}
			if _, err := auth.VerifyCSRFToken(r.Context(), cookie.Value, r.Header.Get("X-CSRF-Token")); err != nil {
				writeError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validOrigin(r *http.Request, allowed []string) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		return true
	}
	origin, err := url.Parse(raw)
	if err != nil || origin.Scheme == "" || origin.Host == "" {
		return false
	}
	actual := strings.ToLower(origin.Scheme + "://" + origin.Host)
	for _, value := range allowed {
		if strings.ToLower(strings.TrimRight(value, "/")) == actual {
			return true
		}
	}
	return false
}

func claimsFromContext(ctx context.Context) identity.SessionClaims {
	claims, _ := ctx.Value(authContextKey{}).(identity.SessionClaims)
	return claims
}

func recoverer(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error("http_panic", zap.Any("panic", recovered), zap.ByteString("stack", debug.Stack()))
					writeError(w, r, context.Canceled)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func requestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(capture, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			log.Info("http_request", zap.String("method", r.Method), zap.String("route", route), zap.Int("status", capture.status), zap.Duration("duration", time.Since(start)))
		})
	}
}

type responseCapture struct {
	http.ResponseWriter
	status int
}

func (w *responseCapture) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
