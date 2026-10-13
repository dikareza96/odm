package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := Guard("s3cret", ok)
	do := func(method, target string, hdr map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(`{"url":"https://x/a.exe","open_when_done":true}`))
		r.Host = "127.0.0.1:8721"
		for k, v := range hdr {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	// The drive-by case: a page POSTs a download with no token.
	if c := do("POST", "/api/downloads", nil).Code; c != http.StatusUnauthorized {
		t.Fatalf("no token: %d, want 401", c)
	}
	// Even with a valid token, a foreign Origin is refused.
	if c := do("POST", "/api/downloads", map[string]string{"X-ODM-Token": "s3cret", "Origin": "https://evil.example"}).Code; c != http.StatusForbidden {
		t.Fatalf("foreign origin: %d, want 403", c)
	}
	if c := do("POST", "/api/downloads", map[string]string{"X-ODM-Token": "nope"}).Code; c != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d, want 401", c)
	}
	if c := do("POST", "/api/downloads", map[string]string{"X-ODM-Token": "s3cret", "Origin": "http://127.0.0.1:8721"}).Code; c != http.StatusTeapot {
		t.Fatalf("same origin + token: %d, want pass-through", c)
	}
	if c := do("GET", "/api/downloads", map[string]string{"Cookie": TokenCookie + "=s3cret"}).Code; c != http.StatusTeapot {
		t.Fatalf("cookie token: %d, want pass-through", c)
	}

	// Token bootstrap: sets a strict cookie and redirects without the query.
	w := do("GET", "/?token=s3cret", nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("bootstrap: %d %q", w.Code, w.Header().Get("Location"))
	}
	if sc := w.Header().Get("Set-Cookie"); !strings.Contains(sc, "SameSite=Strict") || !strings.Contains(sc, "HttpOnly") {
		t.Fatalf("cookie flags: %q", sc)
	}
	if c := do("GET", "/?token=bad", nil).Code; c != http.StatusUnauthorized {
		t.Fatalf("bad bootstrap token: %d, want 401", c)
	}
	// Static assets stay reachable so the page can load and bootstrap.
	if c := do("GET", "/app.js", nil).Code; c != http.StatusTeapot {
		t.Fatalf("static: %d, want pass-through", c)
	}
}
