package httpserver

import (
	"net/http"
	"time"

	appauth "example.com/project-template/internal/controller/application/auth"
)

func (h handler) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.auth.Register(r.Context(), appauth.RegisterInput{Email: body.Email, Password: body.Password, DisplayName: body.DisplayName})
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, result.SessionToken)
	writeJSON(w, http.StatusCreated, map[string]any{"user": mapUser(result.User)})
}

func (h handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := h.auth.Login(r.Context(), appauth.LoginInput{Email: body.Email, Password: body.Password})
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, result.SessionToken)
	writeJSON(w, http.StatusOK, map[string]any{"user": mapUser(result.User)})
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
	user, err := h.auth.Me(r.Context(), claimsFromContext(r.Context()).UserID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": mapUser(user)})
}

func (h handler) setSessionCookie(w http.ResponseWriter, token string) {
	ttl := h.cookie.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	http.SetCookie(w, &http.Cookie{Name: h.cookie.Name, Value: token, Path: "/", HttpOnly: true, Secure: h.cookie.Secure, SameSite: http.SameSiteStrictMode, MaxAge: int(ttl.Seconds()), Expires: time.Now().Add(ttl).UTC()})
}

func (h handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: h.cookie.Name, Value: "", Path: "/", HttpOnly: true, Secure: h.cookie.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()})
}
