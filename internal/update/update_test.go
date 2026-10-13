package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		tag, cur string
		want     bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"v0.1.1", "0.1.0", true},
		{"v1.0.0", "0.9.9", true},
		{"v0.1.0", "0.1.0", false},
		{"v0.1.0", "0.1.0-debug", false}, // debug build of the same release
		{"v0.1.0", "0.2.0", false},
		{"v0.10.0", "0.9.0", true}, // numeric, not lexical
		{"v0.2", "0.1.9", true},
		{"latest", "0.1.0", false}, // malformed tag never prompts
		{"v0.2.0", "dev", false},
	}
	for _, c := range cases {
		if got := Newer(c.tag, c.cur); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.tag, c.cur, got, c.want)
		}
	}
}

func TestLatest(t *testing.T) {
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotUA = r.URL.Path, r.Header.Get("User-Agent")
		if r.URL.Path == "/repos/none/none/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://github.com/o/r/releases/tag/v0.2.0","body":"notes"}`))
	}))
	defer srv.Close()

	r, err := Latest(context.Background(), srv.Client(), srv.URL, "o/r")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/o/r/releases/latest" || gotUA == "" {
		t.Fatalf("request path %q, UA %q", gotPath, gotUA)
	}
	if r.Tag != "v0.2.0" || r.Notes != "notes" || r.URL == "" {
		t.Fatalf("release %+v", r)
	}
	if _, err := Latest(context.Background(), srv.Client(), srv.URL, "none/none"); err == nil {
		t.Fatal("404 should be an error")
	}
}
