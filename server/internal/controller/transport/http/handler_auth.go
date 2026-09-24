package httpserver

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"example.com/project-template/internal/controller/application/apperror"
	appoauth "example.com/project-template/internal/controller/application/oauth"
)

func (h handler) startMobileGitLabOAuth(w http.ResponseWriter, r *http.Request) {
	result, err := h.auth.StartMobile(r.Context(), appoauth.StartMobileInput{
		CodeChallenge: r.URL.Query().Get("codeChallenge"),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	http.Redirect(w, r, result.AuthorizationURL, http.StatusFound)
}

func (h handler) exchangeMobileGitLabOAuth(w http.ResponseWriter, r *http.Request) {
	var input appoauth.CompleteMobileInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, r, apperror.Malformed("request body must be valid JSON"))
		return
	}
	result, err := h.auth.CompleteMobile(r.Context(), input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, result.SessionToken, time.Now().UTC().Add(h.cookie.TTL))
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func mobileOAuthFallback(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(strings.TrimSpace(`<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>Open SitLab</title><body><main><h1>Continue in SitLab</h1>
<p>Install or open the SitLab mobile app, then start sign-in again. No authorization details are displayed on this page.</p>
</main></body></html>`)))
}

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
