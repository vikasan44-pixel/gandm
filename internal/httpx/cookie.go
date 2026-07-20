package httpx

import (
	"net/http"
	"time"
)

// RefreshCookieName is the httpOnly cookie carrying the refresh token. Because
// it is httpOnly it is never readable by JavaScript, so an XSS cannot
// exfiltrate it (unlike a token kept in localStorage).
const RefreshCookieName = "gandm_refresh"

// refreshCookiePath scopes the cookie to the API. Combined with same-origin
// deployment, SameSite=Strict means the cookie is never sent on cross-site
// requests, which neutralizes CSRF on the refresh/logout endpoints without a
// separate CSRF token (all other mutations authenticate via the Bearer header,
// which is not sent automatically cross-site either).
const refreshCookiePath = "/api"

// SetRefreshCookie writes the rotating refresh token as an httpOnly cookie that
// expires together with the token itself.
func SetRefreshCookie(w http.ResponseWriter, token string, secure bool, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearRefreshCookie expires the refresh cookie (logout, or a refresh that
// failed and left the session dead).
func ClearRefreshCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// RefreshCookie reads the refresh token from the request, if present.
func RefreshCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie(RefreshCookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}
