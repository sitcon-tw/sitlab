package httpserver

import (
	"crypto/subtle"
	"net/http"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
	appoauth "example.com/project-template/internal/controller/application/oauth"
)

func (h handler) startGitLabOAuth(w http.ResponseWriter, r *http.Request) {
	result, err := h.auth.Start(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.setOAuthStateCookie(w, result.StateToken)
	http.Redirect(w, r, result.AuthorizationURL, http.StatusFound)
}

func (h handler) completeGitLabOAuth(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie(h.oauthStateCookieName())
	if err != nil || stateCookie.Value == "" || state == "" ||
		subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		writeError(w, r, apperror.Unauthorized("AUTH_OAUTH_FAILED", "OAuth state is invalid or was started in another browser"))
		return
	}
	h.clearOAuthStateCookie(w)
	result, err := h.auth.Complete(r.Context(), appoauth.CompleteInput{
		Code: r.URL.Query().Get("code"), State: state,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, result.SessionToken, time.Now().UTC().Add(h.cookie.TTL))
	http.Redirect(w, r, result.RedirectPath, http.StatusFound)
}

func (h handler) oauthStateCookieName() string { return h.cookie.Name + "_oauth_state" }

func (h handler) setOAuthStateCookie(w http.ResponseWriter, state string) {
	ttl := h.cookie.OAuthStateTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	http.SetCookie(w, &http.Cookie{
		Name: h.oauthStateCookieName(), Value: state, Path: "/", HttpOnly: true,
		Secure: h.cookie.Secure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(ttl.Seconds()), Expires: time.Now().UTC().Add(ttl),
	})
}

func (h handler) clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: h.oauthStateCookieName(), Value: "", Path: "/", HttpOnly: true,
		Secure: h.cookie.Secure, SameSite: http.SameSiteLaxMode,
		MaxAge: -1, Expires: time.Unix(1, 0).UTC(),
	})
}

func (h handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(h.cookie.Name); err == nil {
		if err := h.auth.Logout(r.Context(), cookie.Value); err != nil {
			writeError(w, r, err)
			return
		}
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h handler) csrf(w http.ResponseWriter, r *http.Request) {
	token, err := h.auth.IssueCSRF(r.Context(), claimsFromContext(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.Me(r.Context(), actorID(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": mapUser(user)})
}

func (h handler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	ttl := h.cookie.TTL
	if ttl <= 0 {
		ttl = 14 * 24 * time.Hour
	}
	http.SetCookie(w, &http.Cookie{
		Name: h.cookie.Name, Value: token, Path: "/", HttpOnly: true,
		Secure: h.cookie.Secure, SameSite: http.SameSiteStrictMode,
		MaxAge: int(ttl.Seconds()), Expires: expiresAt.UTC(),
	})
}

func (h handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: h.cookie.Name, Value: "", Path: "/", HttpOnly: true,
		Secure: h.cookie.Secure, SameSite: http.SameSiteStrictMode,
		MaxAge: -1, Expires: time.Unix(1, 0).UTC(),
	})
}
