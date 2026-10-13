package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// TokenCookie carries the session token for the browser UI.
const TokenCookie = "odm_token"

// Guard protects the optional TCP listener (-serve). Loopback TCP is
// reachable by every web page the user visits, so:
//
//   - every /api/ request must present the token, from the SameSite=Strict
//     cookie (the browser UI) or the X-ODM-Token header (scripts);
//   - a request carrying an Origin other than the server's own is refused,
//     which stops cross-site form posts and fetches before any handler runs;
//   - opening "/?token=…" once swaps the query token for the cookie and
//     redirects, so the token does not stay in the address bar.
//
// The Unix-socket channel does not use Guard: only local processes that can
// open the socket file reach it.
func Guard(token string, h http.Handler) http.Handler {
	want := []byte(token)
	valid := func(got string) bool {
		return got != "" && subtle.ConstantTimeCompare([]byte(got), want) == 1
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		if q := r.URL.Query().Get("token"); q != "" && r.Method == http.MethodGet {
			if !valid(q) {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name: TokenCookie, Value: token, Path: "/",
				HttpOnly: true, SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			got := r.Header.Get("X-ODM-Token")
			if c, err := r.Cookie(TokenCookie); got == "" && err == nil {
				got = c.Value
			}
			if !valid(got) {
				http.Error(w, "missing or invalid token", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
